package release

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/helmwave/go-fsimpl"
	"github.com/helmwave/helmwave/pkg/cache"
	"github.com/helmwave/helmwave/pkg/helper"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
)

// Chart is a structure for chart download options.
//
//nolint:lll
type Chart struct {
	Name                  string `yaml:"name" json:"name" jsonschema:"required,description=Name of the chart,example=bitnami/nginx,example=oci://ghcr.io/helmwave/unit-test-oci"`
	CaFile                string `yaml:"ca_file,omitempty" json:"ca_file,omitempty" jsonschema:"description=Verify certificates of HTTPS-enabled servers using this CA bundle"`
	CertFile              string `yaml:"cert_file,omitempty" json:"cert_file,omitempty" jsonschema:"description=Identify HTTPS client using this SSL certificate file"`
	KeyFile               string `yaml:"key_file,omitempty" json:"key_file,omitempty" jsonschema:"description=Identify HTTPS client using this SSL key file"`
	Keyring               string `yaml:"keyring,omitempty" json:"keyring,omitempty" jsonschema:"description=Location of public keys used for verification"`
	RepoURL               string `yaml:"repo_url,omitempty" json:"repo_url,omitempty" jsonschema:"description=Chart repository url"`
	Username              string `yaml:"username,omitempty" json:"username,omitempty" jsonschema:"description=Chart repository username"`
	Password              string `yaml:"password,omitempty" json:"password,omitempty" jsonschema:"description=Chart repository password"`
	Version               string `yaml:"version,omitempty" json:"version,omitempty" jsonschema:"description=Chart version"`
	InsecureSkipTLSverify bool   `yaml:"insecure,omitempty" json:"insecure,omitempty" jsonschema:"description=Connect to server with an insecure way by skipping certificate verification"`
	Verify                bool   `yaml:"verify,omitempty" json:"verify,omitempty" jsonschema:"description=Verify the provenance of the chart before using it"`
	PassCredentialsAll    bool   `yaml:"pass_credentials,omitempty" json:"pass_credentials,omitempty" jsonschema:"description=Pass credentials to all domains"`
	SkipDependencyUpdate  bool   `yaml:"skip_dependency_update" json:"skip_dependency_update" jsonschema:"description=Skip updating and downloading dependencies,default=false"`
	SkipRefresh           bool   `yaml:"skip_refresh,omitempty" json:"skip_refresh,omitempty" jsonschema:"description=Skip refreshing repositories,default=false"`
}

// CopyOptions is a helper for copy options from Chart to ChartPathOptions.
func (c *Chart) CopyOptions(cpo *action.ChartPathOptions) {
	// I hate private field without normal New(...Options)
	cpo.CaFile = c.CaFile
	cpo.CertFile = c.CertFile
	cpo.KeyFile = c.KeyFile
	cpo.InsecureSkipTLSverify = c.InsecureSkipTLSverify
	cpo.Keyring = c.Keyring
	cpo.Password = c.Password
	cpo.PassCredentialsAll = c.PassCredentialsAll
	cpo.RepoURL = c.RepoURL
	cpo.Username = c.Username
	cpo.Verify = c.Verify
	cpo.Version = c.Version
}

// UnmarshalYAML flexible config.
func (u *Chart) UnmarshalYAML(node *yaml.Node) error {
	type raw Chart
	var err error

	switch node.Kind {
	case yaml.ScalarNode, yaml.AliasNode:
		err = node.Decode(&(u.Name))
	case yaml.MappingNode:
		err = node.Decode((*raw)(u))
	default:
		err = ErrUnknownFormat
	}

	if err != nil {
		return fmt.Errorf("failed to decode chart %q from YAML at %d line: %w", node.Value, node.Line, err)
	}

	return nil
}

func (u *Chart) IsRemote(baseFS fs.FS) bool {
	return !helper.IsExists(baseFS, filepath.Clean(u.Name))
}

func (u *Chart) IsLocalArchive(baseFS fs.FS) bool {
	return !helper.IsDir(baseFS, u.Name)
}

func (rel *config) LocateChartWithCache(baseFS fsimpl.CurrentPathFS) (string, error) {
	plandirPath := baseFS.CurrentPath()

	c := rel.Chart()
	ch, err := cache.ChartsCache.FindInCache(c.Name, c.Version)
	if err == nil {
		rel.Logger().Infof("❎ use cache for chart %s: %s", c.Name, ch)

		return ch, nil
	}

	// nice action bro
	client := rel.newInstall()

	chartName := c.Name
	if !c.IsRemote(baseFS) {
		chartName = helper.FilepathJoin(plandirPath, c.Name)
	}

	ch, err = client.ChartPathOptions.LocateChart(chartName, rel.Helm())
	if err != nil {
		return "", fmt.Errorf("failed to locate chart %s: %w", chartName, err)
	}

	cache.ChartsCache.AddToCache(baseFS, ch)

	return ch, nil
}

func (rel *config) GetChart(baseFS fsimpl.CurrentPathFS) (*chart.Chart, error) {
	ch, err := rel.LocateChartWithCache(baseFS)
	if err != nil {
		return nil, err
	}

	c, err := loader.Load(ch)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart %s: %w", rel.Chart().Name, err)
	}

	if err := rel.chartCheck(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (rel *config) chartCheck(ch *chart.Chart) error {
	if req := ch.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(ch, req); err != nil {
			return fmt.Errorf("failed to check chart %s dependencies: %w", ch.Name(), err)
		}
	}

	if !(ch.Metadata.Type == "" || ch.Metadata.Type == "application") {
		rel.Logger().Warnf("%s charts are not installable", ch.Metadata.Type)
	}

	if ch.Metadata.Deprecated {
		rel.Logger().Warnf("⚠️ Chart %s is deprecated. Please update your chart.", ch.Name())
	}

	return nil
}

func (rel *config) ChartDepsUpd(baseFS fsimpl.CurrentPathFS) error {
	plandirPath := baseFS.CurrentPath()

	if rel.Chart().IsRemote(baseFS) {
		rel.Logger().Info("❎ skipping updating dependencies for remote chart")

		return nil
	}

	if rel.Chart().IsLocalArchive(baseFS) {
		rel.Logger().Debug("❎ skipping updating dependencies for downloaded chart")

		return nil
	}

	if rel.Chart().SkipDependencyUpdate {
		rel.Logger().Info("❎ forced skipping updating dependencies for local chart")

		return nil
	}

	settings := rel.Helm()

	client := action.NewDependency()
	man := &downloader.Manager{
		Out:              log.StandardLogger().Writer(),
		ChartPath:        filepath.Clean(helper.FilepathJoin(plandirPath, rel.Chart().Name)),
		Keyring:          client.Keyring,
		SkipUpdate:       rel.Chart().SkipRefresh,
		Getters:          getter.All(settings),
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		Debug:            settings.Debug,
	}
	if client.Verify {
		man.Verify = downloader.VerifyAlways
	}

	if err := man.Update(); err != nil {
		return fmt.Errorf("failed to update %s chart dependencies: %w", rel.Chart().Name, err)
	}

	return nil
}

func (rel *config) DownloadChart(baseFS fsimpl.CurrentPathFS, tmpFS fsimpl.WriteableFS, destDir string) error {
	if !rel.Chart().IsRemote(baseFS) {
		rel.Logger().Info("❎ chart is local, skipping exporting")

		return nil
	}

	if err := tmpFS.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("failed to create temporary directory for chart: %w", err)
	}

	ch, err := rel.LocateChartWithCache(baseFS)
	if err != nil {
		return err
	}

	return helper.CopyFile(baseFS, tmpFS, ch, destDir)
}

func (rel *config) SetChartName(name string) {
	rel.lock.Lock()
	rel.ChartF.Name = name
	rel.lock.Unlock()
}
