import { convertToParamMap } from '@angular/router';
import { BehaviorSubject, of, Subject } from 'rxjs';
import { KubePlatformComponent } from './kube-platform.component';

describe('Investigation workspace state', () => {
  let component: KubePlatformComponent;
  let service: any;
  let route: any;
  let router: any;
  let params: BehaviorSubject<any>;
  let diagnostics: Subject<any>;
  let events: Subject<any>;
  const finding = { id: 'finding-1', name: 'missing-image', topResourceType: 'Deployment',
    rootCause: { name: 'ContainerWaitingAsImagePullBackOff', tags: { reason: 'ImagePullBackOff' } } };
  function navigate(query: any): void {
    route.snapshot.queryParams = query;
    params.next(convertToParamMap(query));
  }
  beforeEach(() => {
    diagnostics = new Subject();
    events = new Subject();
    params = new BehaviorSubject(convertToParamMap({ cluster: 'local', namespace: 'demo', finding: 'finding-1' }));
    route = { queryParamMap: params, paramMap: of(convertToParamMap({})),
      snapshot: { queryParams: { cluster: 'local', namespace: 'demo', finding: 'finding-1' } } };
    router = { navigate: jasmine.createSpy('navigate').and.returnValue(Promise.resolve(true)) };
    service = {
      getClusters: jasmine.createSpy('getClusters').and.returnValue(of(['local', 'other'])),
      getAllNamespaces: jasmine.createSpy('getAllNamespaces').and.returnValue(of(['demo', 'other'])),
      getDetects: jasmine.createSpy('getDetects').and.returnValue(diagnostics),
      getKubeEvents: jasmine.createSpy('getKubeEvents').and.returnValue(events)
    };
    component = new KubePlatformComponent(service, route, router);
    component.ngOnInit();
  });
  afterEach(() => component.ngOnDestroy());

  it('restores a shared finding after asynchronously loading its scope', () => {
    expect(component.selected).toBeUndefined();
    expect(component.loading).toBe(true);
    diagnostics.next([finding]);
    expect(component.selected?.name).toBe('missing-image');
    expect(component.loading).toBe(false);
    expect(component.checked).toBe(true);
    expect(component.lastChecked).not.toBeNull();
  });
  it('restores legacy issue links after loading data', () => {
    component.ngOnDestroy();
    route.paramMap = of(convertToParamMap({ id: 'finding-1' }));
    navigate({ cluster: 'local', namespace: 'demo' });
    component = new KubePlatformComponent(service, route, router);
    component.ngOnInit();
    diagnostics.next([finding]);
    expect(component.selected?.id).toBe('finding-1');
  });
  it('cancels old requests and clears old data when the namespace changes', () => {
    diagnostics.next([finding]);
    events.next([{ Reason: 'old event' }]);
    const nextDiagnostics = new Subject();
    const nextEvents = new Subject();
    service.getDetects.and.returnValue(nextDiagnostics);
    service.getKubeEvents.and.returnValue(nextEvents);
    navigate({ cluster: 'local', namespace: 'other' });
    expect(diagnostics.observers.length).toBe(0);
    expect(events.observers.length).toBe(0);
    expect(component.findings).toEqual([]);
    expect(component.events).toEqual([]);
    expect(component.lastChecked).toBeNull();
    diagnostics.next([finding]);
    expect(component.findings).toEqual([]);
    nextDiagnostics.next([]);
    expect(component.checked).toBe(true);
  });
  it('stops loading on failure without reporting a healthy namespace', () => {
    diagnostics.error({ status: 503 });
    expect(component.loading).toBe(false);
    expect(component.checked).toBe(false);
    expect(component.detectionError).toContain('unavailable');
  });
  it('keeps successful findings when events fail and marks evidence unavailable', () => {
    diagnostics.next([finding]);
    events.error({ status: 403 });
    expect(component.selected?.name).toBe('missing-image');
    expect(component.eventsError).toContain('current access');
    expect(component.loadingEvents).toBe(false);
  });
  it('retains dated results after an unsuccessful refresh', () => {
    diagnostics.next([finding]);
    const previous = component.lastChecked;
    const failedRefresh = new Subject();
    service.getDetects.and.returnValue(failedRefresh);
    component.refresh();
    failedRefresh.error({ status: 502 });
    expect(component.findings.length).toBe(1);
    expect(component.lastChecked).toBe(previous);
    expect(component.detectionError).not.toBe('');
  });
  it('does not refetch when switching tabs or searching within the same scope', () => {
    diagnostics.next([finding]);
    navigate({ cluster: 'local', namespace: 'demo', tab: 'evidence', q: 'missing' });
    expect(component.tab).toBe('evidence');
    expect(component.visibleFindings.length).toBe(1);
    expect(service.getDetects).toHaveBeenCalledTimes(1);
  });
  it('shows a missing shared finding instead of silently selecting another one', () => {
    diagnostics.next([{ ...finding, id: 'different-finding' }]);
    expect(component.visibleFindings.length).toBe(1);
    expect(component.selected).toBeUndefined();
  });
  it('clears a namespace when navigating to a different cluster', () => {
    component.changeCluster('other');
    expect(router.navigate).toHaveBeenCalledWith(['/kubernetes/overview'], { queryParams: { cluster: 'other' } });
  });
  it('treats a null Go slice as an empty successful result', () => {
    diagnostics.next(null);
    expect(component.checked).toBe(true);
    expect(component.findings).toEqual([]);
    expect(component.detectionError).toBe('');
  });
  it('does not interpret an unexpected response object as healthy', () => {
    diagnostics.next({ message: 'unavailable' });
    expect(component.checked).toBe(false);
    expect(component.loading).toBe(false);
    expect(component.detectionError).toContain('could not be read');
  });
  it('sorts events by last occurrence and ignores zero timestamps', () => {
    events.next([
      { Reason: 'old', DateHappened: '2026-10-14T10:00:00Z', LastTimestamp: '0001-01-01T00:00:00Z' },
      { Reason: 'recent', DateHappened: '2026-10-14T09:00:00Z', LastTimestamp: '2026-10-14T12:00:00Z' }
    ]);
    expect(component.events.map(event => event.Reason)).toEqual(['recent', 'old']);
  });
});
