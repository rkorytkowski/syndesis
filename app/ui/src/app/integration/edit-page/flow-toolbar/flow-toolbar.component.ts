import {Component, ElementRef, ViewChild, Input, OnInit} from '@angular/core';
import { CurrentFlowService } from '../current-flow.service';
import { FlowPageService } from '../flow-page.service';
import { ActivatedRoute } from '@angular/router';
import { INTEGRATION_SET_PROPERTY } from '../edit-page.models';

@Component({
  selector: 'syndesis-integration-flow-toolbar',
  templateUrl: './flow-toolbar.component.html',
  styleUrls: ['../../integration-common.scss', './flow-toolbar.component.scss'],
})
export class FlowToolbarComponent implements OnInit {
  @Input() hideButtons = false;
  @ViewChild('nameInput') nameInput: ElementRef;
  private targetUrl: string;

  constructor(
    public currentFlowService: CurrentFlowService,
    public flowPageService: FlowPageService,
    public route: ActivatedRoute
  ) {}

  ngOnInit() {
    this.route.queryParamMap.subscribe(params => this.targetUrl = params.get('targetUrl'));
  }

  get saveInProgress() {
    return this.flowPageService.saveInProgress;
  }

  get publishInProgress() {
    return this.flowPageService.publishInProgress;
  }

  nameUpdated(name: string) {
    this.currentFlowService.events.emit({
      kind: INTEGRATION_SET_PROPERTY,
      property: 'name',
      value: name,
    });
  }

  save(targetRoute: string[]) {
    this.flowPageService.save(
      this.route.firstChild,
      targetRoute || ['..', 'save-or-add-step'], this.targetUrl
    );
  }

  publish() {
    this.flowPageService.publish(this.route.firstChild);
  }

  get currentStep() {
    return this.flowPageService.getCurrentStep(this.route);
  }
}
