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

import com.fasterxml.jackson.databind.ObjectMapper;
import io.atlasmap.v2.Field;
import io.atlasmap.xml.v2.XmlComplexType;
import io.atlasmap.xml.v2.XmlDocument;
import org.apache.commons.io.IOUtils;
import org.assertj.core.api.Assertions;
import org.junit.Test;

import java.io.InputStream;
import java.nio.file.FileSystems;
import java.nio.file.Files;
import java.nio.file.Path;

@SuppressWarnings({"PMD.SignatureDeclareThrowsException", "PMD.JUnitTestsShouldIncludeAssert"})
public class FhirMetadataRetrievalTest {

    @Test
    public void includeResourcesInBundle() throws Exception {
        Path bundle = FileSystems.getDefault().getPath("target/classes/META-INF/syndesis/schemas/dstu3/bundle.json");

        String inspection;
        try (InputStream fileIn = Files.newInputStream(bundle)) {
            inspection = IOUtils.toString(fileIn);
        }

        String inspectionWithResources = new FhirMetadataRetrieval().includeResources(inspection, "patient", "account");

        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Bundle/tns:entry/tns:resource/tns:Patient\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Bundle/tns:entry/tns:resource/tns:Patient/tns:contained/tns:Patient\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Bundle/tns:entry/tns:resource/tns:Patient/tns:contained/tns:Account\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Bundle/tns:entry/tns:resource/tns:Account\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Bundle/tns:entry/tns:resource/tns:Account/tns:contained/tns:Patient\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Bundle/tns:entry/tns:resource/tns:Account/tns:contained/tns:Account\"");

        ObjectMapper mapper = io.atlasmap.v2.Json.mapper();
        XmlDocument xmlDocument = mapper.readValue(inspectionWithResources, XmlDocument.class);

        XmlComplexType resource = (XmlComplexType) xmlDocument.getFields().getField().get(0);
        Assertions.assertThat(resource.getName()).isEqualTo("tns:Bundle");
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:resource", "tns:Patient")).isTrue();
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:resource", "tns:Patient", "tns:contained", "tns:Account")).isTrue();
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:response", "tns:outcome", "tns:Patient")).isTrue();
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:response", "tns:outcome", "tns:Patient", "tns:contained", "tns:Account")).isTrue();
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:resource", "tns:Account")).isTrue();
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:resource", "tns:Account", "tns:contained", "tns:Patient")).isTrue();
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:response", "tns:outcome", "tns:Account")).isTrue();
        Assertions.assertThat(hasPath(resource, "tns:entry", "tns:response", "tns:outcome", "tns:Account", "tns:contained", "tns:Patient")).isTrue();
    }

    @Test
    public void includeResourcesInPatient() throws Exception {
        Path patient = FileSystems.getDefault().getPath("target/classes/META-INF/syndesis/schemas/dstu3/patient.json");

        String inspection;
        try (InputStream fileIn = Files.newInputStream(patient)) {
            inspection = IOUtils.toString(fileIn);
        }

        String inspectionWithResources = new FhirMetadataRetrieval().includeResources(inspection, "person", "account");

        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Patient/tns:contained/tns:Person\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Patient/tns:contained/tns:Person/tns:contained/tns:Account\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Patient/tns:contained/tns:Person/tns:contained/tns:Person\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Patient/tns:contained/tns:Account\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Patient/tns:contained/tns:Account/tns:contained/tns:Account\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Patient/tns:contained/tns:Account/tns:contained/tns:Person\"");
        Assertions.assertThat(inspectionWithResources).containsSequence("\"path\":\"/tns:Patient/tns:contained/tns:Patient\"");

        ObjectMapper mapper = io.atlasmap.v2.Json.mapper();
        XmlDocument xmlDocument = mapper.readValue(inspectionWithResources, XmlDocument.class);

        XmlComplexType resource = (XmlComplexType) xmlDocument.getFields().getField().get(0);
        Assertions.assertThat(resource.getName()).isEqualTo("tns:Patient");
        Assertions.assertThat(hasPath(resource,"tns:contained", "tns:Person")).isTrue();
        Assertions.assertThat(hasPath(resource,"tns:contained", "tns:Person", "tns:contained", "tns:Person")).isTrue();
        Assertions.assertThat(hasPath(resource,"tns:contained", "tns:Person", "tns:contained", "tns:Account")).isTrue();
        Assertions.assertThat(hasPath(resource,"tns:contained", "tns:Account")).isTrue();
        Assertions.assertThat(hasPath(resource,"tns:contained", "tns:Account", "tns:contained", "tns:Person")).isTrue();
        Assertions.assertThat(hasPath(resource,"tns:contained", "tns:Account", "tns:contained", "tns:Account")).isTrue();
    }

    public boolean hasPath(XmlComplexType fields, String... path) {
        return hasPath(fields, 0, path);
    }

    private boolean hasPath(XmlComplexType fields, int depth, String... path) {
        for (Field field: fields.getXmlFields().getXmlField()) {
            if (field instanceof XmlComplexType) {
                XmlComplexType xmlComplexType = (XmlComplexType) field;
                if (xmlComplexType.getName().equals(path[depth])) {
                    if (depth == path.length - 1) {
                        return true;
                    } else {
                        return hasPath(xmlComplexType, depth + 1, path);
                    }
                }
            }
        }
        return false;
    }
}
