/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package configuration

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	consolev1 "github.com/openshift/api/console/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imdario/mergo"
	errs "github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/util/yaml"
	syaml "sigs.k8s.io/yaml"

	"github.com/syndesisio/syndesis/install/operator/pkg/apis/syndesis/v1beta1"
	"github.com/syndesisio/syndesis/install/operator/pkg/syndesis/capabilities"
	"github.com/syndesisio/syndesis/install/operator/pkg/syndesis/clienttools"
	"github.com/syndesisio/syndesis/install/operator/pkg/util"
)

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// Location from where the template configuration is located
var TemplateConfig string

var log = logf.Log.WithName("configuration")

type Config struct {
	AllowLocalHost             bool
	Productized                bool
	Version                    string                     // Syndesis version
	DevSupport                 bool                       // If set to true, pull docker images from imagetag instead of upstream source
	Scheduled                  bool                       // Legacy parameter to set scheduled:true in the imagestreams, but we dont use many imagestreams nowadays
	ProductName                string                     // Usually syndesis or fuse-online
	PrometheusRules            string                     // If some extra rules for prometheus need to be specified, they are defined here
	OpenShiftProject           string                     // The name of the OpenShift project Syndesis is being deployed into
	OpenShiftOauthClientSecret string                     // OpenShift OAuth client secret
	OpenShiftConsoleURL        string                     // The URL to the OpenShift console
	ImagePullSecrets           []string                   // Pull secrets attached to services accounts. This field is generated by the operator
	DatabaseNeedsUpgrade       bool                       // Enabled the image running the database doesn't match the operator's configured image spec
	ApiServer                  capabilities.ApiServerSpec // Metadata of the API Server providing the application
	Syndesis                   SyndesisConfig             // Configuration for syndesis components and addons. This fields are overwritten from environment variables and from the custom resource
}

type SyndesisConfig struct {
	DemoData      bool           // Enables starting up with demo data
	SHA           bool           // Whether we use SHA reference for docker images. If false, tag are used instead
	RouteHostname string         // The external hostname to access Syndesis
	Components    ComponentsSpec // Server, Meta, Ui, Name specifications and configurations
	Addons        AddonsSpec     // Addons specifications and configurations
}

// Components
type ComponentsSpec struct {
	UI         UIConfiguration         // Configuration ui
	S2I        S2IConfiguration        // Configuration s2i
	Oauth      OauthConfiguration      // Configuration oauth
	Server     ServerConfiguration     // Configuration server
	Meta       MetaConfiguration       // Configuration meta
	Database   DatabaseConfiguration   // Configuration database
	Prometheus PrometheusConfiguration // Configuration monitoring
	Grafana    GrafanaConfiguration    // Configuration grafana
	Upgrade    UpgradeConfiguration    // Configuration upgrade
	AMQ        AMQConfiguration        // Configuration AMQ
}

type AMQConfiguration struct {
	Image string // Docker image for AMQ
}

type OauthConfiguration struct {
	Image                 string            // Docker image for proxy
	NonEmbeddedImage      string            // Alternative image for non-embedded auth provider platform
	CookieSecret          string            // Secret to use to encrypt oauth cookies
	DisableSarCheck       bool              // Enable or disable SAR checks all together
	SarNamespace          string            // The user needs to have permissions to at least get a list of pods in the given project in order to be granted access to the Syndesis installation
	CredentialsSecret     string            // The name of the secret used to store provider credentials
	CredentialsSecretData map[string][]byte // The data of the credentials secret
	CryptoCommsSecret     string            // The name of the secret used to provide the TLS certificate for secure communication
}

type UIConfiguration struct {
	Image string // Docker image for UI
}

type S2IConfiguration struct {
	Image string // Docker image for S2I
}

type DatabaseConfiguration struct {
	Image            string                        // Docker image for Database
	User             string                        // Username for PostgreSQL user that will be used for accessing the database
	Name             string                        // Name of the PostgreSQL database accessed
	URL              string                        // Host and port of the PostgreSQL database to access
	ExternalDbURL    string                        // If specified, use an external database instead of the installed by syndesis
	Resources        ResourcesWithPersistentVolume // Resources, memory and database volume size
	Exporter         ExporterConfiguration         // The exporter exports metrics in prometheus format
	Password         string                        // Password for the PostgreSQL connection user
	SampledbPassword string                        // Password for the PostgreSQL sampledb user
}

type ExporterConfiguration struct {
	Image string // Docker image for database exporter
}

type PrometheusConfiguration struct {
	Image     string              // Docker image for prometheus
	Rules     string              // Monitoring rules for prometheus
	Resources ResourcesWithVolume // Set volume size for prometheus pod, where metrics are stored
}

type GrafanaConfiguration struct {
	Resources Resources // Resources for grafana pod, memory
}

type ServerConfiguration struct {
	Image                        string         // Docker image for syndesis server
	Resources                    Resources      // Resources reserved for server pod
	Features                     ServerFeatures // Server features: integration limits and check interval, support for demo data and more
	SyndesisEncryptKey           string         // The encryption key used to encrypt/decrypt stored secrets
	ClientStateAuthenticationKey string         // Key used to perform authentication of client side stored state
	ClientStateEncryptionKey     string         // Key used to perform encryption of client side stored state
}

type MetaConfiguration struct {
	Image     string                        // Docker image for syndesis meta
	Resources ResourcesWithPersistentVolume // Resources for meta pod, memory
}

type UpgradeConfiguration struct {
	Image     string              // Docker image for upgrade pod
	Resources VolumeOnlyResources // Resources for upgrade pod, memory and volume size where database dump is saved
}

type Resources struct {
	Memory string
}

type ResourcesWithVolume struct {
	Memory         string
	VolumeCapacity string
}

type ResourcesWithPersistentVolume struct {
	Memory             string
	VolumeCapacity     string
	VolumeName         string
	VolumeAccessMode   string
	VolumeStorageClass string
	VolumeLabels       map[string]string
}

type VolumeOnlyResources struct {
	VolumeCapacity string
}

type ServerFeatures struct {
	IntegrationLimit              int                // Maximum number of integrations single user can create
	IntegrationStateCheckInterval int                // Interval for checking the state of the integrations
	DeployIntegrations            bool               // Whether we deploy integrations
	TestSupport                   bool               // Enables test-support endpoint on backend API
	OpenShiftMaster               string             // Public OpenShift master address
	ManagementURLFor3scale        string             // 3scale management URL
	Maven                         MavenConfiguration // Maven settings
}

type MavenConfiguration struct {
	Append              bool              // Should we append new repositories
	AdditionalArguments string            // User can set extra maven options
	Repositories        map[string]string // Set repositories for maven
}

// Addons
type AddonsSpec struct {
	Jaeger    JaegerConfiguration
	Ops       OpsConfiguration
	Todo      TodoConfiguration
	Knative   KnativeConfiguration
	CamelK    CamelKConfiguration
	PublicAPI PublicAPIConfiguration
}

type JaegerConfiguration struct {
	Enabled       bool // Whether the addon is enabled
	Olm           OlmSpec
	ClientOnly    bool
	OperatorOnly  bool
	QueryURI      string
	CollectorURI  string
	SamplerType   string
	SamplerParam  string
	ImageAgent    string
	ImageAllInOne string
	ImageOperator string
}

func (j JaegerConfiguration) Name() string {
	return "jaeger"
}
func (j JaegerConfiguration) IsEnabled() bool {
	return j.Enabled
}
func (j JaegerConfiguration) GetOlmSpec() *OlmSpec {
	if j.ClientOnly {
		//
		// If client only then we really don't want to try and install
		// an operator at all, inc. using the operatorhub. As we check
		// for an olm object to determine if the operatorhub
		// should be used, we return nil here to stop installation.
		//
		return nil
	}

	return &j.Olm
}

type OpsConfiguration struct {
	AddonConfiguration
}

func (o OpsConfiguration) Name() string {
	return "ops"
}

type TodoConfiguration struct {
	Image string // Docker image for todo sample app
	AddonConfiguration
}

func (t TodoConfiguration) Name() string {
	return "todo"
}

type KnativeConfiguration struct {
	AddonConfiguration
}

func (k KnativeConfiguration) Name() string {
	return "knative"
}

type PublicAPIConfiguration struct {
	AddonConfiguration
	RouteHostname   string
	DisableSarCheck bool
}

func (p PublicAPIConfiguration) Name() string {
	return "publicApi"
}

type CamelKConfiguration struct {
	Image string
	AddonConfiguration
	CamelVersion  string
	CamelKRuntime string
}

func (c CamelKConfiguration) Name() string {
	return "camelk"
}

type AddonConfiguration struct {
	Enabled bool    // Whether the addon is enabled
	Olm     OlmSpec // The specification for the Operator-Lifecyle-Manager
}

type OlmSpec struct {
	Package string // The name of the package if available in the operator-lifecycle-manager
	Channel string // The preferred channel from which to take the operator
}

func (ac AddonConfiguration) IsEnabled() bool {
	return ac.Enabled
}
func (ac AddonConfiguration) GetOlmSpec() *OlmSpec {
	return &ac.Olm
}

type AddonInfo interface {
	Name() string
	IsEnabled() bool
	GetOlmSpec() *OlmSpec
}

const (
	SyndesisGlobalConfigSecret = "syndesis-global-config"
)

// matches anything followed by space followed by number.number followed (optionally) by another .number and an optional space
// meant to parse strings like "PostgreSQL 9.5.14" to "9.5" and "postgres (PostgreSQL) 10.6 (Debian 10.6-1.pgdg90+1)" to "10.6"
var postgresVersionRegex = regexp.MustCompile(`^.* (\d+\.\d+)(?:\.d+)? ?`)

/*
/ Returns an array of the addons metadata
*/
func GetAddonsInfo(configuration Config) []AddonInfo {
	return []AddonInfo{
		configuration.Syndesis.Addons.Jaeger,
		configuration.Syndesis.Addons.Ops,
		configuration.Syndesis.Addons.CamelK,
		configuration.Syndesis.Addons.Knative,
		configuration.Syndesis.Addons.PublicAPI,
		configuration.Syndesis.Addons.Todo,
	}
}

//
// Route resources set the RouteHostname if not defined by the CR
// but ingresses will not so check the CR has provided one
//
func (config *Config) CheckRouteHostname() error {
	if config.ApiServer.Routes {
		return nil
	}

	if len(config.Syndesis.RouteHostname) == 0 {
		return errors.New("The operator configuration requires a route hostname be defined.")
	}

	return nil
}

func findSecret(ctx context.Context, rtClient client.Client, secretName string, namespace string) (*corev1.Secret, error) {
	if len(secretName) == 0 {
		return nil, fmt.Errorf("The operator is expecting a name of a secret but none has not been specified.")
	}

	secret := &corev1.Secret{}
	err := rtClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: secretName}, secret)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("The authentication secret %s has not been installed", secretName)
		} else {
			return nil, err
		}
	}
	return secret, nil
}

type ByName []corev1.EnvVar

func (k ByName) Len() int           { return len(k) }
func (k ByName) Swap(i, j int)      { k[i], k[j] = k[j], k[i] }
func (k ByName) Less(i, j int) bool { return k[i].Name < k[j].Name }

func SecretToEnvVars(secretName string, secretData map[string][]byte, indents int) (string, error) {
	envVars := make([]corev1.EnvVar, 0)
	for key, _ := range secretData {
		envVar := corev1.EnvVar{
			Name: key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: key,
				},
			},
		}

		envVars = append(envVars, envVar)

	}

	// Sort the environment variables
	sort.Sort(ByName(envVars))

	// Marshal the slice to yaml
	data, err := syaml.Marshal(envVars)
	if err != nil {
		return "", err
	}

	indent := ""
	for i := 0; i < indents; i++ {
		indent = indent + "  "
	}

	// Read the new yaml string & indent each line to required length
	indentData := ""
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		if len(indentData) == 0 {
			indentData = fmt.Sprintf("%s%s\n", indent, scanner.Text())
		} else {
			indentData = fmt.Sprintf("%s%s%s\n", indentData, indent, scanner.Text())
		}
	}

	return indentData, nil
}

//
// If provider is embedded then the OAuth credentials set to the internal
// authentication server so no provider is required. Therefore,
// no provider, clientId or clientSecret is required.
//
func (config *Config) CheckOAuthCredentialSecret(ctx context.Context, rtClient client.Client, syndesis *v1beta1.Syndesis) error {
	if config.ApiServer.EmbeddedProvider {
		return nil
	}

	// Check credentials secret is present
	if secret, err := findSecret(ctx, rtClient, config.Syndesis.Components.Oauth.CredentialsSecret, syndesis.Namespace); err != nil {
		return errs.Wrap(err, "Failed to find the oauth credentials secret")
	} else {
		config.Syndesis.Components.Oauth.CredentialsSecretData = secret.Data
	}

	// Check crypto tls secret is present
	if _, err := findSecret(ctx, rtClient, config.Syndesis.Components.Oauth.CryptoCommsSecret, syndesis.Namespace); err != nil {
		return errs.Wrap(err, "Failed to find the oauth TLS secret")
	}

	return nil
}

/*
/ Returns all processed configurations for Syndesis

 - Default values for configuration are loaded from file
 - Secrets and passwords are loaded from syndesis-global-config Secret if they exits
 and generated if they dont
 - For QE, some fields are loaded from environment variables
 - Users might define fields using the syndesis custom resource
*/
func GetProperties(ctx context.Context, file string, clientTools *clienttools.ClientTools, syndesis *v1beta1.Syndesis) (*Config, error) {
	configuration := &Config{}
	if err := configuration.loadFromFile(file); err != nil {
		return nil, err
	}

	var rtClient client.Client
	var err error
	if clientTools != nil {
		rtClient, err = clientTools.RuntimeClient()
		if err != nil {
			return nil, err
		}
	}

	if ac, err := capabilities.ApiCapabilities(clientTools); err != nil {
		return nil, err
	} else {
		configuration.ApiServer = *ac
	}

	if err := configuration.loadFromFile(file); err != nil {
		return nil, err
	}

	configuration.OpenShiftProject = syndesis.Namespace
	configuration.Syndesis.Components.Oauth.SarNamespace = configuration.OpenShiftProject

	if rtClient != nil {
		if err := configuration.setPasswordsFromSecret(ctx, rtClient, syndesis); err != nil {
			return nil, err
		}
	}
	configuration.generatePasswords()

	if err := configuration.setConfigFromEnv(); err != nil {
		return nil, err
	}

	if err := configuration.setSyndesisFromCustomResource(syndesis); err != nil {
		return nil, err
	}

	//
	// If an external database has been defined then reset properties appropriately
	//
	if rtClient != nil && len(syndesis.Spec.Components.Database.ExternalDbURL) > 0 {
		if err := configuration.externalDatabase(ctx, rtClient, syndesis); err != nil {
			return nil, err
		}
	}

	return configuration, nil
}

// Load configuration from config file. Config file is expected to be a yaml
// The returned configuration is parsed to JSON and returned as a Config object
func (config *Config) loadFromFile(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	if strings.HasSuffix(file, ".yaml") || strings.HasSuffix(file, ".yml") {
		data, err = yaml.ToJSON(data)
		if err != nil {
			return err
		}
	}

	if err := json.Unmarshal(data, config); err != nil {
		return err
	}

	return nil
}

// Set Config.RouteHostname based on the Spec.Host property of the syndesis route
// If an environment variable is set to overwrite the route, take that instead
func (config *Config) SetRoute(ctx context.Context, specRouteHostname string) error {
	if os.Getenv("ROUTE_HOSTNAME") == "" {
		config.Syndesis.RouteHostname = specRouteHostname
		if err := config.CheckRouteHostname(); err != nil {
			return err
		}
	} else {
		config.Syndesis.RouteHostname = os.Getenv("ROUTE_HOSTNAME")
	}
	return nil
}

// When an external database is defined, reset connection parameters
func (config *Config) externalDatabase(ctx context.Context, client client.Client, syndesis *v1beta1.Syndesis) error {
	// Handle an external database being defined
	if syndesis.Spec.Components.Database.ExternalDbURL != "" {

		// setup connection string from provided url
		externalDbURL, err := url.Parse(syndesis.Spec.Components.Database.ExternalDbURL)
		if err != nil {
			return err
		}
		if externalDbURL.Path == "" {
			externalDbURL.Path = syndesis.Spec.Components.Database.Name
		}

		config.Syndesis.Components.Database.URL = externalDbURL.String()
	}

	return nil
}

func getSyndesisConfigurationSecret(ctx context.Context, client client.Client, namespace string) (*corev1.Secret, error) {
	secret := corev1.Secret{}
	if err := client.Get(ctx, util.NewObjectKey(SyndesisGlobalConfigSecret, namespace), &secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func (config *Config) setPasswordsFromSecret(ctx context.Context, client client.Client, syndesis *v1beta1.Syndesis) error {
	if client == nil {
		return nil
	}

	secret, err := getSyndesisConfigurationSecret(ctx, client, syndesis.Namespace)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	/*
	 * If none exist in the secret then config property is set to ""
	 * If this is the case then passwords are generated as a result of
	 * the call to generatePasswords() following execution of this function
	 */
	if _, ok := secret.Data["POSTGRESQL_PASSWORD"]; !ok {
		// This is an indicator that the secret has the old format. We need to extract the
		// secrets from the `params` section instead
		// TODO: Delete for 1.10
		envFromSecret, err := getSyndesisEnvVarsFromOpenShiftNamespace(secret)
		if err != nil {
			return err
		}

		config.OpenShiftOauthClientSecret = envFromSecret["OPENSHIFT_OAUTH_CLIENT_SECRET"]
		config.Syndesis.Components.Database.Password = envFromSecret["POSTGRESQL_PASSWORD"]
		config.Syndesis.Components.Database.SampledbPassword = envFromSecret["POSTGRESQL_SAMPLEDB_PASSWORD"]
		config.Syndesis.Components.Oauth.CookieSecret = envFromSecret["OAUTH_COOKIE_SECRET"]
		config.Syndesis.Components.Server.SyndesisEncryptKey = envFromSecret["SYNDESIS_ENCRYPT_KEY"]
		config.Syndesis.Components.Server.ClientStateAuthenticationKey = envFromSecret["CLIENT_STATE_AUTHENTICATION_KEY"]
		config.Syndesis.Components.Server.ClientStateEncryptionKey = envFromSecret["CLIENT_STATE_ENCRYPTION_KEY"]
	} else {
		// This is the behaviour we want
		config.OpenShiftOauthClientSecret = string(secret.Data["OPENSHIFT_OAUTH_CLIENT_SECRET"])
		config.Syndesis.Components.Database.Password = string(secret.Data["POSTGRESQL_PASSWORD"])
		config.Syndesis.Components.Database.SampledbPassword = string(secret.Data["POSTGRESQL_SAMPLEDB_PASSWORD"])
		config.Syndesis.Components.Oauth.CookieSecret = string(secret.Data["OAUTH_COOKIE_SECRET"])
		config.Syndesis.Components.Server.SyndesisEncryptKey = string(secret.Data["SYNDESIS_ENCRYPT_KEY"])
		config.Syndesis.Components.Server.ClientStateAuthenticationKey = string(secret.Data["CLIENT_STATE_AUTHENTICATION_KEY"])
		config.Syndesis.Components.Server.ClientStateEncryptionKey = string(secret.Data["CLIENT_STATE_ENCRYPTION_KEY"])
	}

	return nil
}

// Overwrite operand images with values from ENV if those env are present
func (config *Config) setConfigFromEnv() error {
	imgEnv := Config{
		Syndesis: SyndesisConfig{
			Addons: AddonsSpec{
				CamelK: CamelKConfiguration{Image: os.Getenv("RELATED_IMAGE_CAMELK")},
				Todo:   TodoConfiguration{Image: os.Getenv("RELATED_IMAGE_TODO")},
			},
			Components: ComponentsSpec{
				Oauth:      OauthConfiguration{Image: os.Getenv("RELATED_IMAGE_OAUTH")},
				UI:         UIConfiguration{Image: os.Getenv("RELATED_IMAGE_UI")},
				S2I:        S2IConfiguration{Image: os.Getenv("RELATED_IMAGE_S2I")},
				Prometheus: PrometheusConfiguration{Image: os.Getenv("RELATED_IMAGE_PROMETHEUS")},
				Upgrade:    UpgradeConfiguration{Image: os.Getenv("RELATED_IMAGE_UPGRADE")},
				Meta: MetaConfiguration{
					Image: os.Getenv("RELATED_IMAGE_META"),
					Resources: ResourcesWithPersistentVolume{
						VolumeAccessMode:   os.Getenv("META_VOLUME_ACCESS_MODE"),
						VolumeStorageClass: os.Getenv("META_STORAGE_CLASS"),
						VolumeName:         os.Getenv("META_VOLUME_NAME"),
					},
				},
				Database: DatabaseConfiguration{
					Image:    os.Getenv("RELATED_IMAGE_DATABASE"),
					Exporter: ExporterConfiguration{Image: os.Getenv("RELATED_IMAGE_PSQL_EXPORTER")},
					Resources: ResourcesWithPersistentVolume{
						VolumeAccessMode:   os.Getenv("DATABASE_VOLUME_ACCESS_MODE"),
						VolumeStorageClass: os.Getenv("DATABASE_STORAGE_CLASS"),
						VolumeName:         os.Getenv("DATABASE_VOLUME_NAME"),
					},
				},
				Server: ServerConfiguration{
					Image: os.Getenv("RELATED_IMAGE_SERVER"),
				},
				AMQ: AMQConfiguration{Image: os.Getenv("RELATED_IMAGE_AMQ")},
			},
		},
	}

	if err := mergo.Merge(config, imgEnv, mergo.WithOverride); err != nil {
		return err
	}

	config.DevSupport = setBoolFromEnv("DEV_SUPPORT", config.DevSupport)
	config.Syndesis.Components.Server.Features.TestSupport = setBoolFromEnv("TEST_SUPPORT", config.Syndesis.Components.Server.Features.TestSupport)

	return nil
}

// Return the value of a config given its default value and an environment
// variable.
func setBoolFromEnv(env string, current bool) bool {
	var result bool
	if varFromEnv := os.Getenv(env); varFromEnv != "" {
		result = varFromEnv == "true"
	} else {
		result = current
	}

	return result
}

// Return the value of a config given its default value and an environment
// variable.
func setIntFromEnv(env string, current int) int {
	if varFromEnv := os.Getenv(env); varFromEnv != "" {
		if result, err := strconv.Atoi(varFromEnv); err == nil {
			return result
		}
	}

	return current
}

// Replace default values with those from custom resource
func (config *Config) setSyndesisFromCustomResource(syndesis *v1beta1.Syndesis) error {
	c := SyndesisConfig{}
	jsonProperties, err := json.Marshal(syndesis.Spec)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(jsonProperties, &c); err != nil {
		return err
	}

	if err := mergo.Merge(&config.Syndesis, c, mergo.WithOverride); err != nil {
		return err
	}

	// If specified, we overwrite the maven repositories with cr repositories
	if len(syndesis.Spec.Components.Server.Features.Maven.Repositories) > 0 && !syndesis.Spec.Components.Server.Features.Maven.Append {
		config.Syndesis.Components.Server.Features.Maven.Repositories = syndesis.Spec.Components.Server.Features.Maven.Repositories
	}

	return nil
}

// Generate random expressions for passwords and secrets
func (config *Config) generatePasswords() {

	if config.OpenShiftOauthClientSecret == "" {
		config.OpenShiftOauthClientSecret = generatePassword(64)
	}

	if config.Syndesis.Components.Database.Password == "" {
		config.Syndesis.Components.Database.Password = generatePassword(16)
	}

	if config.Syndesis.Components.Database.SampledbPassword == "" {
		config.Syndesis.Components.Database.SampledbPassword = generatePassword(16)
	}

	if config.Syndesis.Components.Oauth.CookieSecret == "" {
		config.Syndesis.Components.Oauth.CookieSecret = generatePassword(32)
	}

	if config.Syndesis.Components.Server.SyndesisEncryptKey == "" {
		config.Syndesis.Components.Server.SyndesisEncryptKey = generatePassword(64)
	}

	if config.Syndesis.Components.Server.ClientStateAuthenticationKey == "" {
		config.Syndesis.Components.Server.ClientStateAuthenticationKey = generatePassword(32)
	}

	if config.Syndesis.Components.Server.ClientStateEncryptionKey == "" {
		config.Syndesis.Components.Server.ClientStateEncryptionKey = generatePassword(32)
	}
}

func generatePassword(size int) string {
	alphabet := make([]rune, (26*2)+10)
	i := 0
	for c := 'a'; c <= 'z'; c++ {
		alphabet[i] = c
		i++
	}
	for c := 'A'; c <= 'Z'; c++ {
		alphabet[i] = c
		i++
	}
	for c := '0'; c <= '9'; c++ {
		alphabet[i] = c
		i++
	}

	result := make([]rune, size)
	for i := 0; i < size; i++ {
		result[i] = alphabet[random.Intn(len(alphabet))]
	}
	s := string(result)
	return s
}

// Needed for the first run after upgrade, due to compatibilities with old
// secret format
// TODO: Delete for 1.10
func parseConfigurationBlob(blob []byte) map[string]string {
	strs := strings.Split(string(blob), "\n")
	configs := make(map[string]string, 0)
	for _, conf := range strs {
		conf := strings.Trim(conf, " \r\t")
		if conf == "" {
			continue
		}
		kv := strings.SplitAfterN(conf, "=", 2)
		if len(kv) == 2 {
			key := strings.TrimRight(kv[0], "=")
			value := kv[1]
			configs[key] = value
		}
	}
	return configs
}

// TODO: Delete for 1.10
func getSyndesisEnvVarsFromOpenShiftNamespace(secret *corev1.Secret) (map[string]string, error) {
	if envBlob, present := secret.Data["params"]; present {
		return parseConfigurationBlob(envBlob), nil
	}

	return nil, errors.New("no configuration found")
}

func (config *Config) SetConsoleLink(ctx context.Context, client client.Client, syndesis *v1beta1.Syndesis, syndesisRouteHost string) error {
	if syndesisRouteHost == "" {
		return nil
	}

	if !config.ApiServer.ConsoleLink {
		// Cluster does not support ConsoleLink API
		return nil
	}

	consoleLinkName := consoleLinkName(syndesis)
	consoleLink := &consolev1.ConsoleLink{}
	err := client.Get(ctx, types.NamespacedName{Name: consoleLinkName}, consoleLink)
	if err != nil {
		log.Info(consoleLink.Name)
		consoleLink = createNamespaceDashboardLink(consoleLinkName, syndesisRouteHost, syndesis)
		if err := client.Create(ctx, consoleLink); err != nil {
			log.Error(err, "error creating console link")
			return err
		}
	} else if err == nil && consoleLink != nil {
		if syndesis.DeletionTimestamp != nil {
			if err := client.Delete(ctx, consoleLink); err != nil {
				log.Error(err, "Error deleting console link.")
			}
		}

		if err := reconcileConsoleLink(ctx, syndesis, syndesisRouteHost, consoleLink, client); err != nil {
			return err
		}
	}

	return nil
}

func reconcileConsoleLink(ctx context.Context, syndesis *v1beta1.Syndesis, routeHost string, link *consolev1.ConsoleLink, client client.Client) error {
	updateConsoleLink := false
	url := "https://" + routeHost
	if link.Spec.Href != url {
		link.Spec.Href = url
		updateConsoleLink = true
	}

	linkText := syndesis.Name
	if link.Spec.Text != linkText {
		link.Spec.Text = linkText
		updateConsoleLink = true
	}

	if updateConsoleLink {
		if err := client.Update(ctx, link); err != nil {
			return err
		}
	}

	return nil
}

func consoleLinkName(syndesis *v1beta1.Syndesis) string {
	return syndesis.Name + "-" + syndesis.Namespace
}

func createNamespaceDashboardLink(name string, routeHost string, syndesis *v1beta1.Syndesis) *consolev1.ConsoleLink {
	return &consolev1.ConsoleLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: syndesis.Labels,
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Text: name,
				Href: "https://" + routeHost,
			},
			Location: consolev1.NamespaceDashboard,
			NamespaceDashboard: &consolev1.NamespaceDashboardSpec{
				Namespaces: []string{syndesis.Namespace},
			},
		},
	}
}
