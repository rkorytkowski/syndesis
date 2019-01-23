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
package io.syndesis.connector.fhir;

import com.fasterxml.jackson.core.PrettyPrinter;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.atlasmap.xml.v2.XmlComplexType;
import io.atlasmap.xml.v2.XmlDocument;
import io.atlasmap.xml.v2.XmlField;
import io.syndesis.common.model.DataShape;
import io.syndesis.common.model.DataShapeKinds;
import io.syndesis.common.util.Resources;
import io.syndesis.connector.support.verifier.api.ComponentMetadataRetrieval;
import io.syndesis.connector.support.verifier.api.PropertyPair;
import io.syndesis.connector.support.verifier.api.SyndesisMetadata;
import org.apache.camel.CamelContext;
import org.apache.camel.component.extension.MetaDataExtension;
import org.apache.camel.util.ObjectHelper;
import org.apache.commons.lang3.StringUtils;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;

public class FhirMetadataRetrieval extends ComponentMetadataRetrieval {

    final ObjectMapper mapper = io.atlasmap.v2.Json.mapper();

    /**
     * TODO: use local extension, remove when switching to camel 2.22.x
     */
    @Override
    protected MetaDataExtension resolveMetaDataExtension(CamelContext context, Class<? extends MetaDataExtension> metaDataExtensionClass, String componentId, String actionId) {
        return new FhirMetaDataExtension(context);
    }

    @SuppressWarnings("unchecked")
    @Override
    protected SyndesisMetadata adapt(CamelContext context, String componentId, String actionId, Map<String, Object> properties, MetaDataExtension.MetaData metadata) {
        if (!properties.containsKey("resourceType")) {
            return SyndesisMetadata.EMPTY;
        }

        Set<String> resourceTypes = (Set<String>) metadata.getPayload();
        List<PropertyPair> resourceTypeResult = new ArrayList<>();
        resourceTypes.stream().forEach(
            t -> resourceTypeResult.add(new PropertyPair(t, t))
        );

        if (ObjectHelper.isNotEmpty(properties.get("resourceType"))) {
            final Map<String, List<PropertyPair>> enrichedProperties = new HashMap<>();
            enrichedProperties.put("resourceType",resourceTypeResult);
            enrichedProperties.put("containedResourceTypes", resourceTypeResult);

            String type = properties.get("resourceType").toString();
            try {
                String resourcePath = type.toLowerCase(Locale.ENGLISH);
                String specification = Resources.getResourceAsText("META-INF/syndesis/schemas/dstu3/" + resourcePath + ".json", FhirMetadataRetrieval.class.getClassLoader());

                Object containedResourceTypes = properties.get("containedResourceTypes");
                if (ObjectHelper.isNotEmpty(containedResourceTypes)) {
                    //TODO: Fix property to be String[] (multi selection) and not String (single selection)
                    specification = includeResources(specification, (String) containedResourceTypes);
                }

                if (actionId.contains("read")) {
                    return new SyndesisMetadata(
                        enrichedProperties,
                        new DataShape.Builder().kind(DataShapeKinds.JAVA)//
                            .type("io.syndesis.connector.fhir.FhirResourceId")
                            .description("FHIR " + actionId)
                            .name(actionId).build(),
                        new DataShape.Builder().kind(DataShapeKinds.XML_SCHEMA_INSPECTED)//
                            .type(type)
                            .description("FHIR " + type)
                            .specification(specification)
                            .name(type).build());
                } else if (actionId.contains("delete")) {
                    return new SyndesisMetadata(
                        enrichedProperties,
                        new DataShape.Builder().kind(DataShapeKinds.JAVA)//
                            .type("io.syndesis.connector.fhir.FhirResourceId")
                            .description("FHIR " + actionId)
                            .name(actionId).build(),
                        new DataShape.Builder().kind(DataShapeKinds.JAVA)//
                            .type("ca.uhn.fhir.rest.api.MethodOutcome")
                            .description("FHIR " + actionId)
                            .name(actionId).build());
                } else if (actionId.contains("create")) {
                    return new SyndesisMetadata(
                        enrichedProperties,
                        new DataShape.Builder().kind(DataShapeKinds.XML_SCHEMA_INSPECTED)//
                            .type(type)
                            .description("FHIR " + type)
                            .specification(specification)
                            .name(type).build(),
                        new DataShape.Builder().kind(DataShapeKinds.JAVA)//
                            .type("ca.uhn.fhir.rest.api.MethodOutcome")
                            .description("FHIR " + actionId)
                            .name(actionId).build());
                } else if (actionId.contains("update")) {
                    return new SyndesisMetadata(
                        enrichedProperties,
                        new DataShape.Builder().kind(DataShapeKinds.XML_SCHEMA_INSPECTED)//
                            .type(type)
                            .description("FHIR " + type)
                            .specification(specification)
                            .name(type).build(),
                        new DataShape.Builder().kind(DataShapeKinds.JAVA)//
                            .type("ca.uhn.fhir.rest.api.MethodOutcome")
                            .description("FHIR " + actionId)
                            .name(actionId).build());
                }
            } catch (Exception e) {
                throw new IllegalStateException(
                    "Error retrieving resource schema for type: " + type, e);
            }
        } else {
            Map<String, List<PropertyPair>> map = new HashMap<>();
            map.put("resourceType", resourceTypeResult);
            map.put("containedResourceTypes", resourceTypeResult);
            return SyndesisMetadata.of(map);
        }

        return SyndesisMetadata.EMPTY;
    }

    String includeResources(String specification, String... resourceTypes) throws IOException {
        if (resourceTypes != null && resourceTypes.length != 0) {
            XmlDocument document = mapper.readValue(specification, XmlDocument.class);
            includeResources(null, document, resourceTypes);
            specification = mapper.writer((PrettyPrinter) null).writeValueAsString(document);
        }
        return specification;
    }

    private void includeResources(String rootPath, XmlDocument resource, String... resourceTypes) throws IOException {
        XmlComplexType resourceElement = (XmlComplexType) resource.getFields().getField().get(0);
        switch (resourceElement.getName()) {
            case "tns:Bundle":
                includeResourcesInPath(rootPath, resourceElement, resourceTypes, "tns:entry", "tns:resource");
                includeResourcesInPath(rootPath, resourceElement, resourceTypes, "tns:entry", "tns:response", "tns:outcome");
                break;
            case "tns:Parameters":
                includeResourcesInPath(rootPath, resourceElement, resourceTypes, "tns:parameter", "tns:resource");
                break;
            default:
                includeResourcesInPath(rootPath, resourceElement, resourceTypes, "tns:contained");
        }
    }

    private void includeResourcesInPath(String rootPath, XmlComplexType resourceElement, String[] resourceTypes, String... path) throws IOException {
        XmlComplexType element = getElement(resourceElement, 0, path);
        if (element != null) {
            List<XmlField> resourcesToInclude = new ArrayList<>();
            for (String resourceType: resourceTypes) {
                String inspectionToInclude = Resources.getResourceAsText("META-INF/syndesis/schemas/dstu3/" + resourceType.toLowerCase(Locale.ENGLISH) + ".json", FhirMetadataRetrieval.class.getClassLoader());
                String pathToReplace = resourceElement.getName() + "/" + StringUtils.join(path, "/");
                if (rootPath != null) {
                    pathToReplace = rootPath + "/" + pathToReplace;
                }
                inspectionToInclude = inspectionToInclude.replace("\"path\":\"", "\"path\":\"/" + pathToReplace);

                XmlDocument resourceToInclude = mapper.readValue(inspectionToInclude, XmlDocument.class);
                if (rootPath == null) {
                    includeResources(pathToReplace, resourceToInclude, resourceTypes);
                }

                resourcesToInclude.add((XmlField) resourceToInclude.getFields().getField().get(0));
            }

            element.getXmlFields().getXmlField().clear();
            element.getXmlFields().getXmlField().addAll(resourcesToInclude);
        }
    }

    private XmlComplexType getElement(XmlComplexType field, int depth, String... path) {
        for (XmlField xmlField: field.getXmlFields().getXmlField()) {
            if (xmlField instanceof XmlComplexType) {
                XmlComplexType xmlComplexType = (XmlComplexType) xmlField;
                if (xmlComplexType.getName().equals(path[depth])) {
                    if (depth == path.length - 1) {
                        return xmlComplexType;
                    } else {
                        return getElement(xmlComplexType, depth + 1, path);
                    }
                }
            }
        }
        return null;
    }
}
