/* Copyright FMR LLC <opensource@fidelity.com>
 * SPDX-License-Identifier: Apache
 */
import { Component, OnDestroy, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { combineLatest, Subscription } from 'rxjs';
import { timeout } from 'rxjs/operators';
import { faArrowRight, faCheck, faChevronRight, faCircleNotch, faCopy, faCube,
  faExclamationTriangle, faFilter, faSearch, faSyncAlt, faTerminal, faTimes } from '@fortawesome/free-solid-svg-icons';
import { KubernetesService } from '../services/kubernetes.service';
import { DiagnosticReport, Investigation, KubeEvent, matchesFinding, relatedEvents,
  toInvestigation, validTimestamp } from './investigation';
type DetailTab = 'overview' | 'evidence' | 'resources';

@Component({
  selector: 'app-kube-platform',
  templateUrl: './kube-platform.component.html',
  styleUrls: ['./kube-platform.component.scss']
})
export class KubePlatformComponent implements OnInit, OnDestroy {
  icons = { search: faSearch, refresh: faSyncAlt, filter: faFilter, arrow: faArrowRight,
    chevron: faChevronRight, cube: faCube, warning: faExclamationTriangle, check: faCheck,
    spinner: faCircleNotch, copy: faCopy, terminal: faTerminal, close: faTimes };
  clusters: string[] = [];
  namespaces: string[] = [];
  cluster = '';
  namespace = '';
  query = '';
  kindFilter = '';
  reasonFilter = '';
  nameFilter = '';
  filtersOpen = false;
  loadingClusters = false;
  loadingNamespaces = false;
  loading = false;
  loadingEvents = false;
  checked = false;
  clusterError = '';
  namespaceError = '';
  detectionError = '';
  eventsError = '';
  lastChecked: Date | null = null;
  eventsChecked: Date | null = null;
  findings: Investigation[] = [];
  events: KubeEvent[] = [];
  selectedId = '';
  tab: DetailTab = 'overview';
  allEvents = false;
  copyStatus = '';
  private scope = '';
  private subscriptions = new Subscription();
  private namespaceRequest = new Subscription();
  private detectionRequest = new Subscription();
  private eventRequest = new Subscription();
  private clusterRequest = new Subscription();
  private copyTimer?: ReturnType<typeof setTimeout>;

  constructor(private kubeService: KubernetesService, private route: ActivatedRoute, private router: Router) {}

  ngOnInit(): void {
    this.subscriptions.add(combineLatest([this.route.queryParamMap, this.route.paramMap]).subscribe(([query, path]) => {
      const cluster = query.get('cluster') || '';
      const namespace = query.get('namespace') || '';
      this.selectedId = query.get('finding') || path.get('id') || '';
      const requestedTab = query.get('tab');
      this.tab = requestedTab === 'evidence' || requestedTab === 'resources' ? requestedTab : 'overview';
      this.query = query.get('q') || '';
      this.kindFilter = query.get('type') || '';
      this.reasonFilter = query.get('reason') || '';
      this.nameFilter = query.get('name') || '';
      this.filtersOpen = this.filtersOpen || !!(this.kindFilter || this.reasonFilter || this.nameFilter);
      const scope = JSON.stringify([cluster, namespace]);
      if (scope === this.scope) { return; }
      const clusterChanged = cluster !== this.cluster;
      this.scope = scope;
      this.cluster = cluster;
      this.namespace = namespace;
      this.clearResults();
      if (clusterChanged) { this.loadNamespaces(); }
      if (cluster && namespace) { this.refresh(); }
    }));
    this.loadClusters();
  }
  ngOnDestroy(): void {
    this.subscriptions.unsubscribe();
    this.clusterRequest.unsubscribe();
    this.namespaceRequest.unsubscribe();
    this.detectionRequest.unsubscribe();
    this.eventRequest.unsubscribe();
    if (this.copyTimer) { clearTimeout(this.copyTimer); }
  }
  get visibleFindings(): Investigation[] {
    return this.findings.filter(finding => matchesFinding(finding, this.query, this.kindFilter, this.reasonFilter, this.nameFilter));
  }
  get selected(): Investigation | undefined {
    return this.selectedId ? this.visibleFindings.find(finding => finding.id === this.selectedId) : this.visibleFindings[0];
  }
  get kinds(): string[] { return Array.from(new Set(this.findings.map(finding => finding.kind))).sort(); }
  get reasons(): string[] { return Array.from(new Set(this.findings.map(finding => finding.reason))).sort(); }
  get activeFilterCount(): number { return [this.kindFilter, this.reasonFilter, this.nameFilter].filter(Boolean).length; }
  get matchingEvents(): KubeEvent[] { return this.selected ? relatedEvents(this.events, this.selected, this.namespace) : []; }
  get displayedEvents(): KubeEvent[] { return this.allEvents || !this.selected ? this.events : this.matchingEvents; }
  get busy(): boolean { return this.loading || this.loadingEvents; }

  loadClusters(): void {
    this.clusterRequest.unsubscribe();
    this.loadingClusters = true;
    this.clusterError = '';
    this.clusterRequest = this.kubeService.getClusters().pipe(timeout(45000)).subscribe((data: string[]) => {
      this.clusters = Array.isArray(data) ? data : [];
      this.loadingClusters = false;
      if (!this.cluster && this.clusters.length === 1) { this.changeCluster(this.clusters[0]); }
    }, error => {
      this.loadingClusters = false;
      this.clusterError = this.errorMessage(error, 'Cluster list');
    });
  }
  loadNamespaces(): void {
    this.namespaceRequest.unsubscribe();
    this.namespaces = [];
    this.namespaceError = '';
    this.loadingNamespaces = !!this.cluster;
    if (!this.cluster) { return; }
    this.namespaceRequest = this.kubeService.getAllNamespaces(this.cluster).pipe(timeout(45000)).subscribe((data: string[]) => {
      this.namespaces = Array.isArray(data) ? data : [];
      this.loadingNamespaces = false;
    }, error => {
      this.loadingNamespaces = false;
      this.namespaceError = this.errorMessage(error, 'Namespace list');
    });
  }
  changeCluster(cluster: string): void {
    this.router.navigate(['/kubernetes/overview'], { queryParams: { cluster: cluster || null } });
  }
  changeNamespace(namespace: string): void {
    this.router.navigate(['/kubernetes/overview'], { queryParams: { cluster: this.cluster, namespace: namespace || null } });
  }
  private clearResults(): void {
    this.detectionRequest.unsubscribe();
    this.eventRequest.unsubscribe();
    this.findings = [];
    this.events = [];
    this.lastChecked = null;
    this.eventsChecked = null;
    this.checked = false;
    this.loading = false;
    this.loadingEvents = false;
    this.detectionError = '';
    this.eventsError = '';
    this.allEvents = false;
    this.copyStatus = '';
  }
  refresh(): void {
    if (!this.cluster || !this.namespace) { return; }
    this.detectionRequest.unsubscribe();
    this.loading = true;
    this.detectionError = '';
    this.detectionRequest = this.kubeService.getDetects(this.cluster, this.namespace).pipe(timeout(45000))
      .subscribe((data: DiagnosticReport[]) => {
      if (data != null && !Array.isArray(data)) {
        this.loading = false;
        this.detectionError = 'Diagnostic results could not be read. Try checking again.';
        return;
      }
      this.findings = (data || []).map(toInvestigation);
      this.lastChecked = new Date();
      this.checked = true;
      this.loading = false;
    }, error => {
      this.loading = false;
      this.detectionError = this.errorMessage(error, 'Diagnostic checks');
    });
    this.refreshEvents();
  }
  refreshEvents(): void {
    if (!this.cluster || !this.namespace) { return; }
    this.eventRequest.unsubscribe();
    this.eventsError = '';
    this.loadingEvents = true;
    this.eventRequest = this.kubeService.getKubeEvents(this.cluster, this.namespace).pipe(timeout(45000)).subscribe((data: KubeEvent[]) => {
      this.loadingEvents = false;
      if (data != null && !Array.isArray(data)) {
        this.eventsError = 'Event results could not be read. Try loading them again.';
        return;
      }
      this.events = (data || []).slice().sort((a, b) =>
        Date.parse(this.eventTime(b) || '1970-01-01') - Date.parse(this.eventTime(a) || '1970-01-01'));
      this.eventsChecked = new Date();
    }, error => {
      this.loadingEvents = false;
      this.eventsError = this.errorMessage(error, 'Kubernetes events');
    });
  }
  selectFinding(finding: Investigation): void {
    this.allEvents = false;
    this.copyStatus = '';
    this.updateQuery({ finding: finding.id, tab: 'overview' });
  }
  selectTab(tab: DetailTab): void {
    this.updateQuery({ finding: this.selected ? this.selected.id : null, tab });
  }
  filterChanged(): void {
    this.updateQuery({ q: this.query || null, type: this.kindFilter || null, reason: this.reasonFilter || null,
      name: this.nameFilter || null, finding: null, tab: 'overview' }, true);
  }
  clearFilters(): void {
    this.query = this.kindFilter = this.reasonFilter = this.nameFilter = '';
    this.filterChanged();
  }
  showFirstFinding(): void { this.updateQuery({ finding: null, tab: 'overview' }); }
  private updateQuery(params: { [key: string]: string | null }, replaceUrl = false): void {
    this.router.navigate(['/kubernetes/overview'], {
      queryParams: { ...this.route.snapshot.queryParams, cluster: this.cluster || null, namespace: this.namespace || null, ...params },
      replaceUrl
    });
  }
  eventTime(event: KubeEvent): string | null { return validTimestamp(event.LastTimestamp) || validTimestamp(event.DateHappened); }
  eventObject(event: KubeEvent): string {
    const object = event.InvolvedObject || {};
    return [object.Kind || object.kind, object.name || object.Name].filter(Boolean).join(' / ');
  }
  metadataEntries(finding: Investigation): { key: string; value: string }[] {
    return Object.keys(finding.tags).sort().map(key => ({ key, value: finding.tags[key] }));
  }
  async copy(text: string, label: string): Promise<void> {
    if (this.copyTimer) { clearTimeout(this.copyTimer); }
    try {
      await navigator.clipboard.writeText(text);
      this.copyStatus = label + ' copied';
    } catch (_) {
      this.copyStatus = 'Copy unavailable. Select the text to copy it manually.';
    }
    this.copyTimer = setTimeout(() => this.copyStatus = '', 5000);
  }
  copyLink(): void {
    const tree = this.router.createUrlTree(['/kubernetes/overview'], { queryParams: {
      cluster: this.cluster, namespace: this.namespace, finding: this.selected ? this.selected.id : this.selectedId, tab: this.tab
    } });
    this.copy(window.location.href.split('#')[0] + '#' + this.router.serializeUrl(tree), 'Investigation link');
  }
  private errorMessage(error: any, subject: string): string {
    if (error && (error.status === 401 || error.status === 403)) {
      return subject + ' are unavailable with your current access. Check your session and permissions.';
    }
    return subject + ' are unavailable. Check the connection and try again.';
  }
}
