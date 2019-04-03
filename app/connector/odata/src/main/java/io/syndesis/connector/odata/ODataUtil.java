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
package io.syndesis.connector.odata;

import org.apache.camel.support.jsse.KeyManagersParameters;
import org.apache.camel.support.jsse.KeyStoreParameters;
import org.apache.camel.support.jsse.SSLContextParameters;
import org.apache.camel.support.jsse.TrustManagersParameters;
import org.apache.camel.util.ObjectHelper;
import org.apache.http.auth.AuthScope;
import org.apache.http.auth.UsernamePasswordCredentials;
import org.apache.http.client.CredentialsProvider;
import org.apache.http.client.HttpClient;
import org.apache.http.client.methods.HttpGet;
import org.apache.http.conn.ssl.NoopHostnameVerifier;
import org.apache.http.impl.client.BasicCredentialsProvider;
import org.apache.http.impl.client.CloseableHttpClient;
import org.apache.http.impl.client.HttpClientBuilder;
import org.apache.http.impl.nio.client.HttpAsyncClientBuilder;
import org.apache.olingo.client.api.http.HttpClientFactory;
import org.apache.olingo.commons.api.http.HttpMethod;

import javax.net.ssl.HostnameVerifier;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import java.io.ByteArrayInputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.io.InputStream;
import java.net.URI;
import java.nio.charset.Charset;
import java.security.KeyManagementException;
import java.security.KeyStore;
import java.security.KeyStoreException;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.security.UnrecoverableKeyException;
import java.security.cert.Certificate;
import java.security.cert.CertificateException;
import java.security.cert.CertificateFactory;
import java.util.Map;

@SuppressWarnings("PMD")
public class ODataUtil implements ODataConstants {

    private static final String DEFAULT_KEYSTORE_PASSWD = "changeit";

    public static class ODataHttpClientFactory implements HttpClientFactory {

        private final Map<String, Object> options;

        public ODataHttpClientFactory(Map<String, Object> options) {
            this.options = options;
        }

        @Override
        public HttpClient create(HttpMethod method, URI uri) {
            try {
                return createHttpClient(options);
            } catch (Exception e) {
                throw new IllegalStateException(e);
            }
        }

        @SuppressWarnings( "deprecation" )
        @Override
        public void close(HttpClient httpClient) {
            httpClient.getConnectionManager().shutdown();
        }

    }

    /**
     * @param url
     * @return whether url is an ssl (https) url or not.
     */
    public static boolean isServiceSSL(String url) {
        if (url == null) {
            return false;
        }

        HttpGet httpGet = new HttpGet(url);
        String scheme = httpGet.getURI().getScheme();
        return scheme != null && scheme.equals("https");
    }

    private static InputStream defaultKeyStore() throws FileNotFoundException {
        return new FileInputStream(
                                          System.getProperties()
                                                .getProperty("java.home") + File.separator
                                              + "lib" + File.separator + "security" + File.separator
                                              + "cacerts");
    }

    private static KeyStore createKeyStore(Map<String, Object> options)
        throws KeyStoreException, IOException, NoSuchAlgorithmException, CertificateException {

        KeyStore keyStore = KeyStore.getInstance(KeyStore.getDefaultType());
        keyStore.load(defaultKeyStore(), DEFAULT_KEYSTORE_PASSWD.toCharArray());

        String certContent = (String) options.get(SERVER_CERTIFICATE);
        if (certContent != null) {
            Certificate certificate = CertificateFactory.getInstance("X.509")
                .generateCertificate(
                                     new ByteArrayInputStream(certContent.getBytes(Charset.defaultCharset())));
            keyStore.setCertificateEntry("odata", certificate);
        }
        return keyStore;
    }

    public static SSLContext createSSLContext(Map<String, Object> options)
                throws NoSuchAlgorithmException, KeyManagementException,
                                KeyStoreException, IOException, CertificateException {

        String serviceUrl = (String) options.get(SERVICE_URI);
        if (! isServiceSSL(serviceUrl)) {
            return null;
        }

        KeyStore keyStore = createKeyStore(options);
        TrustManagerFactory tmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        tmf.init(keyStore);

        SSLContext sslContext = SSLContext.getInstance("TLS");
        sslContext.init(null, tmf.getTrustManagers(), new SecureRandom());
        return sslContext;
    }

    public static SSLContextParameters createSSLContextParameters(Map<String, Object> options) {
        String serviceUrl = (String) options.get(SERVICE_URI);
        if (! isServiceSSL(serviceUrl)) {
            return null;
        }

        SSLContextParameters sslContextParams = new SSLContextParameters();
        KeyStoreParameters keystoreParams = new KeyStoreParameters() {
            @Override
            protected InputStream resolveResource(String resource) throws IOException {
                return defaultKeyStore();
            };

            @Override
            public String getPassword() {
                return DEFAULT_KEYSTORE_PASSWD;
            }
        };

        KeyManagersParameters keyManagersParams = new KeyManagersParameters();
        keyManagersParams.setKeyStore(keystoreParams);

        TrustManagersParameters trustManagersParams = new TrustManagersParameters();
        trustManagersParams.setKeyStore(keystoreParams);

        SSLContextParameters sslContextParameters = new SSLContextParameters();
        sslContextParameters.setKeyManagers(keyManagersParams);
        sslContextParameters.setTrustManagers(trustManagersParams);
        return sslContextParams;
    }

    private static CredentialsProvider createCredentialProvider(Map<String, Object> options) {
        String basicUser = (String) options.get(BASIC_USER_NAME);
        String basicPswd = (String) options.get(BASIC_PASSWORD);

        if (ObjectHelper.isEmpty(basicUser)) {
            return null;
        }

        CredentialsProvider credentialsProvider = new BasicCredentialsProvider();
        credentialsProvider.setCredentials(AuthScope.ANY, new UsernamePasswordCredentials(basicUser, basicPswd));
        return credentialsProvider;
    }

    /**
     * Creates a new {@link HttpClientBuilder} for the given options.
     *
     * @param options
     *
     * @return the new http client builder
     *
     * @throws CertificateException
     * @throws KeyStoreException
     * @throws NoSuchAlgorithmException
     * @throws KeyManagementException
     * @throws IOException
     * @throws UnrecoverableKeyException
     * @throws Exception
     */
    public static HttpClientBuilder createHttpClientBuilder(Map<String, Object> options)
                                                       throws CertificateException, KeyManagementException,
                                                                      NoSuchAlgorithmException, KeyStoreException, IOException, UnrecoverableKeyException {
        HttpClientBuilder builder = HttpClientBuilder.create();

        SSLContext sslContext = createSSLContext(options);
        if (sslContext != null) {
            // Skip verifying hostname
            HostnameVerifier allowAllHosts = new NoopHostnameVerifier();
            builder.setSSLContext(sslContext);
            builder.setSSLHostnameVerifier(allowAllHosts);
        }

        CredentialsProvider credentialsProvider = createCredentialProvider(options);
        if (credentialsProvider != null) {
            builder.setDefaultCredentialsProvider(credentialsProvider).build();
        }

        return builder;
    }

    /**
     * Creates a new {@link HttpClientBuilder} for the given options.
     *
     * @param options
     *
     * @return the new http client builder
     *
     * @throws CertificateException
     * @throws KeyStoreException
     * @throws NoSuchAlgorithmException
     * @throws KeyManagementException
     * @throws IOException
     * @throws UnrecoverableKeyException
     * @throws Exception
     */
    public static HttpAsyncClientBuilder createHttpAsyncClientBuilder(Map<String, Object> options)
                                                       throws CertificateException, KeyManagementException,
                                                                      NoSuchAlgorithmException, KeyStoreException, IOException, UnrecoverableKeyException {
        HttpAsyncClientBuilder builder = HttpAsyncClientBuilder.create();

        SSLContext sslContext = createSSLContext(options);
        if (sslContext != null) {
            // Skip verifying hostname
            HostnameVerifier allowAllHosts = new NoopHostnameVerifier();
            builder.setSSLContext(sslContext);
            builder.setSSLHostnameVerifier(allowAllHosts);
        }

        CredentialsProvider credentialsProvider = createCredentialProvider(options);
        if (credentialsProvider != null) {
            builder.setDefaultCredentialsProvider(credentialsProvider).build();
        }

        return builder;
    }

    /**
     * Creates a new {@link CloseableHttpClient} for the given options.
     * @param options
     *
     * @return the new http(s) client
     *
     * @throws CertificateException
     * @throws KeyStoreException
     * @throws NoSuchAlgorithmException
     * @throws KeyManagementException
     * @throws IOException
     * @throws UnrecoverableKeyException
     * @throws Exception
     */
    public static CloseableHttpClient createHttpClient(Map<String, Object> options)
                                                                   throws CertificateException, KeyManagementException,
                                                                       NoSuchAlgorithmException, KeyStoreException, IOException, UnrecoverableKeyException {
        return createHttpClientBuilder(options).build();
    }

    public static HttpClientFactory newHttpFactory(Map<String, Object> options) {
        return new ODataHttpClientFactory(options);
    }
}
