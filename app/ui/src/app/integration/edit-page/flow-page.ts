import { OnDestroy } from '@angular/core';
import { Subscription } from 'rxjs/Subscription';
import { ActivatedRoute, Params, Router } from '@angular/router';
import { CurrentFlow, FlowEvent } from '@syndesis/ui/integration/edit-page';
import { Integration, Step } from '@syndesis/ui/platform';

export abstract class FlowPage implements OnDestroy {
  flowSubscription: Subscription;
  errorMessage: any = undefined;
  saveInProgress = false;
  publishInProgress = false;

  constructor(
    public currentFlow: CurrentFlow,
    public route: ActivatedRoute,
    public router: Router
  ) {
    this.flowSubscription = this.currentFlow.events.subscribe(
      (event: FlowEvent) => {
        this.handleFlowEvent(event);
      }
    );
  }

  canContinue() {
    return true;
  }

  get integrationName() {
    return this.currentFlow.integration
      ? this.currentFlow.integration.name
      : undefined;
  }

  cancel() {
    if (this.currentFlow.integration.id) {
      this.router.navigate(['/integrations', this.currentFlow.integration.id]);
    } else {
      this.router.navigate(['/integrations']);
    }
  }

  goBack(path: Array<string | number | boolean>) {
    this.router.navigate(path, { relativeTo: this.route.parent });
  }

  handleFlowEvent(event: FlowEvent) {
    /* no-op */
  }

  doSave() {
    this.errorMessage = undefined;
    if (
      !this.currentFlow.integration.name ||
      this.currentFlow.integration.name === ''
    ) {
      this.router.navigate(['integration-basics'], {
        relativeTo: this.route.parent
      });
      this.saveInProgress = false;
      this.publishInProgress = false;
      return;
    }
    const router = this.router;
    this.currentFlow.events.emit({
      kind: 'integration-save',
      action: (i: Integration) => {
        if (this.saveInProgress) {
          this.saveInProgress = false;
          return;
        }
        const target = i.id ? ['/integrations', i.id] : ['/integrations'];
        this.router.navigate(target);
      }
    });
  }

  save(status: 'Draft' | 'Active' | 'Inactive' | 'Undeployed' = undefined) {
    this.saveInProgress = true;
    this.doSave();
  }

  publish(
    status: 'Draft' | 'Active' | 'Inactive' | 'Undeployed' = 'Active'
  ) {
    this.publishInProgress = true;
    this.doSave();
  }

  ngOnDestroy() {
    if (this.flowSubscription) {
      this.flowSubscription.unsubscribe();
    }
  }
}
