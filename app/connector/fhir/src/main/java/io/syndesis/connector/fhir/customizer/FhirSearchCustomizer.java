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
package io.syndesis.connector.fhir.customizer;

import io.syndesis.integration.component.proxy.ComponentProxyComponent;
import org.apache.camel.Exchange;
import org.apache.camel.Message;
import org.apache.camel.component.fhir.internal.FhirSearchApiMethod;
import org.apache.camel.impl.DefaultMessage;
import org.apache.camel.util.ObjectHelper;
import org.apache.camel.util.component.ApiMethod;

import java.util.Map;

public class FhirSearchCustomizer  extends FhirReadCustomizer {
    private String query;

    @Override
    public Class<? extends ApiMethod> getApiMethodClass() {
        return FhirSearchApiMethod.class;
    }

    @Override
    public void customize(ComponentProxyComponent component, Map<String, Object> options) {
        super.customize(component, options);
        query = (String) options.get("query");

        options.put("methodName", "searchByUrl?inBody=url");
    }


    @Override
    public void beforeProducer(Exchange exchange) {
        final Message in = exchange.getIn();

        if (ObjectHelper.isNotEmpty(query)) {
            in.setBody(query);
        }
    }
}
