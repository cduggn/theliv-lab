/* Copyright FMR LLC <opensource@fidelity.com>
 * SPDX-License-Identifier: Apache
 */
export interface DiagnosticIssue {
  name?: string;
  description?: string;
  solutions?: string[];
  commands?: string[];
  createdTime?: string;
  tags?: { [key: string]: string };
}
export interface DiagnosticResource {
  name: string;
  type: string;
  issue?: DiagnosticIssue;
  metadata?: { [key: string]: any };
  labels?: { [key: string]: string };
  annotations?: { [key: string]: string };
  deeplink?: { [key: string]: string };
}
export interface DiagnosticReport {
  id?: string;
  name: string;
  topResourceType?: string;
  rootCause?: DiagnosticIssue;
  resources?: DiagnosticResource[];
}
export interface KubeEvent {
  EventId?: string;
  Message?: string;
  Reason?: string;
  Type?: string;
  DateHappened?: string;
  LastTimestamp?: string;
  InvolvedObject?: { [key: string]: string };
}
export interface Investigation {
  id: string;
  name: string;
  kind: string;
  title: string;
  reason: string;
  observed: string;
  explanation: string;
  nextCheck: string;
  tags: { [key: string]: string };
  resources: DiagnosticResource[];
  recommendations: string[];
  commands: string[];
  firstObserved: string | null;
}
export function readableName(value: string): string {
  const words = value.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/[_-]+/g, ' ').trim();
  return words ? words.charAt(0).toUpperCase() + words.slice(1) : 'Finding needs investigation';
}
export function validTimestamp(value?: string): string | null {
  return value && !value.startsWith('0001-') && Number.isFinite(Date.parse(value)) ? value : null;
}
// The legacy API returns numbered prose/command blocks. Present each item once,
// and leave numbering out of the text copied into a terminal.
export function splitNumberedBlocks(values: string[]): string[] {
  return values.reduce((items, value) => items.concat(
    value.split(/(?:^|\n)\s*\d+\.\s+/).map(item => item.trim()).filter(Boolean)
  ), [] as string[]);
}
export function toInvestigation(report: DiagnosticReport): Investigation {
  const issue = report.rootCause || {};
  const tags = issue.tags || {};
  const reason = tags.reason || issue.name || 'Unknown';
  const resources = report.resources || [];
  const issues = [issue, ...resources.map(resource => resource.issue || {})];
  const unique = (values: string[]) => Array.from(new Set(values.filter(value => typeof value === 'string' && value.trim())));
  let title = readableName(issue.name || reason);
  let observed = issue.description || 'A diagnostic check reported a finding on this resource.';
  let nextCheck = 'Review the detector details and related events to establish the cause.';
  if (/ImagePullBackOff|ErrImagePull/i.test(reason + ' ' + issue.name)) {
    title = 'Container image could not be pulled';
    observed = `Container${tags.container ? ' “' + tags.container + '”' : ''} is waiting because its image could not be pulled${tags.image ? ': ' + tags.image : '.'}`;
    nextCheck = 'Inspect the pull event for the registry error, then check the image reference, registry access, and pull credentials.';
  } else if (/CrashLoopBackOff/i.test(reason + ' ' + issue.name)) {
    title = 'Container keeps restarting';
    nextCheck = 'Check the previous container logs, exit status, and recent configuration changes.';
  } else if (/OOMKilled/i.test(reason + ' ' + issue.name)) {
    title = 'Container exceeded available memory';
    nextCheck = 'Compare memory usage with the container limit and inspect the termination details.';
  } else if (/Pending|FailedScheduling|Unschedulable/i.test(reason + ' ' + issue.name)) {
    title = 'Workload is waiting to be scheduled';
    nextCheck = 'Review scheduling events, resource requests, node availability, and placement constraints.';
  }
  const kind = readableName(report.topResourceType || 'Resource');
  return {
    id: report.id || kind + '/' + report.name,
    name: report.name, kind, title, reason, observed, nextCheck,
    explanation: issue.description || '', tags, resources,
    recommendations: unique(splitNumberedBlocks(issues.reduce((all, item) => all.concat(item.solutions || []), [] as string[]))),
    commands: unique(splitNumberedBlocks(issues.reduce((all, item) => all.concat(item.commands || []), [] as string[]))),
    firstObserved: validTimestamp(issue.createdTime)
  };
}
export function matchesFinding(finding: Investigation, query: string, kind = '', reason = '', name = ''): boolean {
  const searchable = [finding.name, finding.kind, finding.title, finding.reason, finding.observed,
    ...Object.values(finding.tags), ...finding.resources.map(resource => resource.name)].join(' ').toLowerCase();
  return (!kind || finding.kind.toLowerCase() === kind.toLowerCase()) &&
    (!reason || finding.reason === reason) && (!name || finding.name === name) &&
    query.toLowerCase().trim().split(/\s+/).every(term => searchable.includes(term));
}
export function relatedEvents(events: KubeEvent[], finding: Investigation, namespace: string): KubeEvent[] {
  // Match exact resource identities; similar names can belong to unrelated workloads.
  const identities = [
    { name: finding.name, kind: finding.kind },
    ...finding.resources.filter(resource => resource.type.toLowerCase() !== 'container')
      .map(resource => ({ name: resource.name, kind: resource.type })),
    { name: finding.tags.pod, kind: 'Pod' },
    { name: finding.tags.owner_name, kind: finding.tags.owner_kind }
  ].filter(identity => !!identity.name);
  return events.filter(event => {
    const object = event.InvolvedObject || {};
    const eventNamespace = object.namespace || object.Namespace;
    const name = object.name || object.Name;
    const kind = object.Kind || object.kind;
    return (!eventNamespace || eventNamespace === namespace) && identities.some(identity =>
      identity.name === name && (!kind || !identity.kind || identity.kind.toLowerCase() === kind.toLowerCase()));
  });
}
