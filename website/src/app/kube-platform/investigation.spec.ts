import { matchesFinding, relatedEvents, splitNumberedBlocks, toInvestigation, validTimestamp } from './investigation';

describe('Investigation adapter', () => {
  const report = {
    id: 'image-failure', name: 'missing-image', topResourceType: 'Deployment',
    rootCause: {
      name: 'ContainerWaitingAsImagePullBackOff', description: 'A waiting container was detected.',
      tags: { reason: 'ImagePullBackOff', container: 'demo', image: 'busybox:missing', pod: 'missing-image-ab12-cd34' },
      solutions: ['Check the registry'], commands: ['kubectl get pods']
    },
    resources: [{ name: 'missing-image-ab12-cd34', type: 'pod',
      issue: { solutions: ['Check the registry', 'Check the tag'], commands: ['kubectl get pods', 'kubectl describe pod missing-image-ab12-cd34'] } }]
  };
  it('describes the image-pull symptom without claiming the image is missing', () => {
    const finding = toInvestigation(report);
    expect(finding.title).toBe('Container image could not be pulled');
    expect(finding.observed).toContain('busybox:missing');
    expect(finding.nextCheck).toContain('registry error');
    expect(finding.explanation).toBe(report.rootCause.description);
  });
  it('preserves resource-level advice and removes duplicates', () => {
    const finding = toInvestigation(report);
    expect(finding.recommendations).toEqual(['Check the registry', 'Check the tag']);
    expect(finding.commands.length).toBe(2);
  });
  it('supports reports with no optional details', () => {
    const finding = toInvestigation({ name: 'worker' });
    expect(finding.resources).toEqual([]);
    expect(finding.commands).toEqual([]);
    expect(finding.id).toBe('Resource/worker');
    expect(finding.firstObserved).toBeNull();
  });
  it('searches image references and child resources, with intersecting filters', () => {
    const finding = toInvestigation(report);
    expect(matchesFinding(finding, 'BUSYBOX demo', 'deployment')).toBe(true);
    expect(matchesFinding(finding, 'ab12-cd34')).toBe(true);
    expect(matchesFinding(finding, 'busybox', 'Pod')).toBe(false);
    expect(matchesFinding(finding, '', '', 'CrashLoopBackOff')).toBe(false);
    expect(matchesFinding(finding, '', '', '', 'another-workload')).toBe(false);
  });
  it('matches evidence by exact name, kind and namespace', () => {
    const finding = toInvestigation(report);
    const events = [
      { Reason: 'match', InvolvedObject: { name: 'missing-image-ab12-cd34', namespace: 'demo', Kind: 'Pod' } },
      { Reason: 'other namespace', InvolvedObject: { name: 'missing-image-ab12-cd34', namespace: 'other', Kind: 'Pod' } },
      { Reason: 'similar name', InvolvedObject: { name: 'missing-image-extra', namespace: 'demo', Kind: 'Deployment' } },
      { Reason: 'wrong kind', InvolvedObject: { name: 'missing-image', namespace: 'demo', Kind: 'ConfigMap' } },
      { Reason: 'parent', InvolvedObject: { name: 'missing-image', namespace: 'demo', Kind: 'Deployment' } }
    ];
    expect(relatedEvents(events, finding, 'demo').map(event => event.Reason)).toEqual(['match', 'parent']);
  });
  it('does not display Go zero timestamps or invalid dates', () => {
    expect(validTimestamp('0001-01-01T00:00:00Z')).toBeNull();
    expect(validTimestamp('not-a-date')).toBeNull();
    expect(validTimestamp('2026-10-14T12:00:00Z')).toBe('2026-10-14T12:00:00Z');
  });
  it('makes numbered legacy command blocks copyable without changing shell content', () => {
    expect(splitNumberedBlocks(['\n1. kubectl get pods -n demo\n2. kubectl get events -n demo\n']))
      .toEqual(['kubectl get pods -n demo', 'kubectl get events -n demo']);
    expect(splitNumberedBlocks(['kubectl get pod example -o json |\n  jq .status']))
      .toEqual(['kubectl get pod example -o json |\n  jq .status']);
  });
});
