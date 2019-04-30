/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import {
  Component,
  ViewEncapsulation,
  OnInit,
  OnDestroy, Input, Output, EventEmitter, OnChanges
} from '@angular/core';

import {
  ContentBasedRouter,
  CurrentFlowService, FlowOption, FlowPageService,
  INTEGRATION_ADD_FLOW
} from '@syndesis/ui/integration/edit-page';
import {
  Action,
  ActionDescriptor,
  createStep,
  DataShape,
  DataShapeKinds,
  IntegrationSupportService,
  key,
  Step
} from '@syndesis/ui/platform';
import { ActivatedRoute, Router } from '@angular/router';
import { Subscription } from 'rxjs';
import { FormArray, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ENDPOINT } from '@syndesis/ui/store';
import { ModalService } from '@syndesis/ui/common';

@Component({
  selector: 'syndesis-content-based-router',
  templateUrl: './content-based-router.component.html',
  encapsulation: ViewEncapsulation.None,
  styleUrls: ['./content-based-router.component.scss']
})
export class ContentBasedRouterComponent implements OnInit, OnChanges, OnDestroy {
  form: FormGroup;
  flowOptions: any = [];

  step: Step;
  loading = true;
  formValueChangeSubscription: Subscription;

  @Input()
  configuredProperties: ContentBasedRouter = {
    routingScheme: 'direct',
    default: 'comics',
    flows: [
      {
        condition: '${body.text} contains Spiderman',
        flow: 'spiderman'
      },
      {
        condition: '${body.text} contains Batman',
        flow: 'batman'
      }
    ]
  };
  @Input() valid: boolean;
  @Input() position: number;
  @Output() validChange = new EventEmitter<boolean>();
  @Output() configuredPropertiesChange = new EventEmitter<ContentBasedRouter>();

  constructor(
    private currentFlowService: CurrentFlowService,
    public integrationSupportService: IntegrationSupportService,
    public flowPageService: FlowPageService,
    public route: ActivatedRoute,
    private router: Router,
    private fb: FormBuilder,
    private modalService: ModalService
  ) {
    // nothing to do
  }

  // this can be valid even if we can't fetch the form data
  initForm(
    configuredProperties?: ContentBasedRouter,
  ): void {
    let configuredFlows: FlowOption[] = undefined;
    const configuredFlowGroups = [];
    const configuredDefaultFlow = (configuredProperties && configuredProperties.default)
      ? configuredProperties.default
      : '';

    // build up the form array from the incoming values (if any)
    if (configuredProperties && configuredProperties.flows) {
      // TODO hackity hack
      if (typeof configuredProperties.flows === 'string') {
        configuredFlows = JSON.parse(<any>configuredProperties.flows);
      } else {
        configuredFlows = configuredProperties.flows;
      }

      for (const incomingFlow of configuredFlows) {
        configuredFlowGroups.push(this.fb.group(incomingFlow));
      }
    }
    const preloadedDefaultFlow = this.fb.group({
      defaultFlowEnabled: [configuredDefaultFlow !== '', null],
      defaultFlow: [configuredDefaultFlow, null]
    });

    let preloadedFlowOptions;
    if (configuredFlowGroups.length > 0) {
      preloadedFlowOptions = this.fb.array(configuredFlowGroups);
    } else {
      preloadedFlowOptions = this.fb.array([this.createNewFlowGroup(key())]);
    }

    this.flowOptions = preloadedFlowOptions;

    const formGroupObj = {
      defaultFlow: preloadedDefaultFlow,
      flowOptions: preloadedFlowOptions
    };

    this.form = this.fb.group(formGroupObj);

    this.formValueChangeSubscription = this.form.valueChanges.subscribe(_ => {
      this.valid = this.form.valid;
      this.validChange.emit(this.valid);
    });

    this.loading = false;
  }

  ngOnChanges(changes: any) {
    if (!('position' in changes)) {
      return;
    }

    this.loading = true;
    // Fetch our form data
    this.initForm(this.configuredProperties);
  }

  ngOnInit(): void {
    // setTimeout needed so that the prompt template is available
    // and ExpressionChangedAfterItHasBeenCheckedError is not thrown
    setTimeout(() => {
      if (!this.currentFlowService.isSaved()) {
        this.modalService
          .show('save-cbr-integration-prompt').then(modal => {
          if (modal.result) {
            this.flowPageService.save(this.route);
          } else {
            this.flowPageService.cancel();
          }
        });
      }
    });
    this.step = this.currentFlowService.getStep(this.position);
  }

  ngOnDestroy(): void {
    if (this.formValueChangeSubscription) {
      this.formValueChangeSubscription.unsubscribe();
    }
  }

  addFlowGroup(flowId: string): void {
    const newGroup = <FormGroup>this.createNewFlowGroup(flowId);
    this.flowOptions = this.form.get('flowOptions') as FormArray;
    this.flowOptions.push(newGroup);
  }

  createNewFlowGroup(flowId: string): FormGroup {
    const group = {
      flow: flowId,
      condition: ['', Validators.compose([
        Validators.required,
        Validators.maxLength(100)
      ])
      ]
    };
    return this.fb.group(group);
  }

  get myFlows(): FormArray {
    return <FormArray>this.flowOptions;
  }

  toggleDefaultFlow() {
    if (this.form.controls.defaultFlow.get('defaultFlowEnabled').value) {
      this.createDefaultFlow();
    } else {
      this.removeDefaultFlow();
    }
  }

  createDefaultFlow() {
    this.doCreateFlow(flowId => this.form.controls.defaultFlow.get('defaultFlow').setValue(flowId));
  }

  removeDefaultFlow(): void {
    this.form.controls.defaultFlow.get('defaultFlow').setValue('');
    this.onChange();
  }

  removeFlow(index: number): void {
    this.myFlows.removeAt(index);
    this.onChange();
  }

  openDefaultFlow() {
    const flowId = this.configuredProperties.default;
    const integrationId = this.currentFlowService.integration.id;
    this.router.navigate([
      '/integrations',
      integrationId,
      flowId,
      'edit'
    ]);
  }

  openFlow(index: number) {
    const flowId = this.myFlows.controls[index].get('flow').value;
    const integrationId = this.currentFlowService.integration.id;
    this.router.navigate([
      '/integrations',
      integrationId,
      flowId,
      'edit'
    ]);
  }

  createFlow() {
    this.doCreateFlow(flowId => this.addFlowGroup(flowId));
  }

  doCreateFlow(then: (flowId: string) => void) {
    const currentFlow = this.currentFlowService.currentFlow;
    const mainFlowId = currentFlow.id;
    const newFlowId = key();
    const targetFlowName = 'From ' + (currentFlow.name || mainFlowId);
    this.currentFlowService.events.emit({
      kind: INTEGRATION_ADD_FLOW,
      flow: {
        name: targetFlowName,
        id: newFlowId,
        steps: [
          this.createFlowStart({
            ...createStep(),
            name: 'Flow start',
            stepKind: ENDPOINT,
            connection: undefined,
            action: {
              actionType: 'connector',
              descriptor: {
                componentScheme: 'direct',
                inputDataShape: {
                  kind: DataShapeKinds.ANY,
                  name: 'Any input shape'
                } as DataShape,
                outputDataShape: {
                  kind: DataShapeKinds.ANY,
                  name: 'Any output shape'
                } as DataShape
              } as ActionDescriptor
            } as Action,
            configuredProperties: {
              name: newFlowId
            }
          }),
          this.createFlowEnd({
            ...createStep(),
            name: 'Flow end',
            stepKind: ENDPOINT,
            connection: undefined,
            action: {
              actionType: 'connector',
              descriptor: {
                componentScheme: 'mock',
                inputDataShape: {
                  kind: DataShapeKinds.ANY,
                  name: 'Any input shape'
                } as DataShape,
                outputDataShape: {
                  kind: DataShapeKinds.ANY,
                  name: 'Any output shape'
                } as DataShape
              } as ActionDescriptor
            } as Action,
            configuredProperties: {
              name: 'flow-end'
            }
          })
        ],
        metadata: {
          mainFlowId: mainFlowId,
          type: 'cbr-flow'
        }
      },
      onSave: () => {
        then(newFlowId);
        this.onChange();
      }
    });
  }

  createFlowStart(step: Step): Step {
    if (this.step &&
      this.step.action &&
      this.step.action.descriptor &&
      this.step.action.descriptor.outputDataShape) {
      step.action.descriptor.outputDataShape = this.step.action.descriptor.outputDataShape;
    }

    return step;
  }

  createFlowEnd(step: Step): Step {
    if (this.step &&
      this.step.action &&
      this.step.action.descriptor &&
      this.step.action.descriptor.inputDataShape) {
      step.action.descriptor.inputDataShape = this.step.action.descriptor.inputDataShape;
    }

    return step;
  }

  onChange() {
    this.valid = this.form.valid;
    this.validChange.emit(this.valid);
    if (!this.valid) {
      return;
    }

    const formGroupObj = this.form.value;

    const formattedProperties: ContentBasedRouter = {
      routingScheme: 'direct',
      default: this.form.controls.defaultFlow.get('defaultFlow').value,
      flows: formGroupObj.flowOptions
    };

    this.configuredPropertiesChange.emit(formattedProperties);
  }
}
