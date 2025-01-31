package main

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/OffchainLabs/methodical-ssz/specs"
	"github.com/OffchainLabs/methodical-ssz/sszgen"
	"github.com/OffchainLabs/methodical-ssz/sszgen/backend"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/urfave/cli/v2"
)

var releaseURI, configPath string
var tests = &cli.Command{
	Name:  "spectest",
	Usage: "generate go test methods to execute spectests against generated types",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "release-uri",
			Usage:       "url or file in file:// format pointing at a github.com/ethereum/consensus-spec-tests release",
			Destination: &releaseURI,
		},
		&cli.StringFlag{
			Name:        "config",
			Usage:       "path to yaml file configuring spec test relationships, see readme or prysm example for format",
			Required:    true,
			Destination: &configPath,
		},
		&cli.StringFlag{
			Name:        "output",
			Usage:       "path to output directory where spec test package and copy of consensus types and ssz methods will be written",
			Required:    true,
			Destination: &output,
		},
	},
	Action: func(c *cli.Context) error {
		return actionSpectests(c)
	},
}

func actionSpectests(cl *cli.Context) error {
	if err := os.MkdirAll(output, os.ModePerm); err != nil {
		return errors.Wrapf(err, "failed to create output directory %s", output)
	}
	fs := afero.NewBasePathFs(afero.NewOsFs(), output)
	cfg, err := specs.ParseConfigFile(configPath)
	if err != nil {
		return err
	}
	types := cfg.GoTypes()
	parser, err := sszgen.NewPackageParser(cfg.Package, types)
	if err != nil {
		return err
	}

	r, err := loadArchive(releaseURI)
	if err != nil {
		return errors.Wrapf(err, "failed to open spectest archive from uri %s", releaseURI)
	}
	cases, err := specs.ExtractCases(r, specs.TestIdent{Preset: specs.Mainnet})
	if err != nil {
		return err
	}
	for ident, _ := range cases {
		fmt.Printf("%s\n", ident)
	}

	g := backend.NewGenerator(cfg.Package, cfg.Package)
	for _, s := range parser.TypeDefs() {
		fmt.Printf("Generating methods for %s/%s\n", s.PackageName, s.Name)
		typeRep, err := sszgen.ParseTypeDef(s)
		if err != nil {
			return err
		}
		g.Generate(typeRep)
	}
	rbytes, err := g.Render()
	if err != nil {
		return err
	}
	if err := afero.WriteFile(fs, "methodical.ssz.go", rbytes, 0666); err != nil {
		return err
	}
	source, err := parser.TypeDefSourceCode()
	if err != nil {
		return err
	}
	if err := afero.WriteFile(fs, "structs.go", source, 0666); err != nil {
		return err
	}
	return specs.WriteSpecTestFiles(cases, cfg, fs)
}

func loadArchive(uri string) (io.Reader, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "file" {
		return os.Open(u.Path)
	}
	return nil, errors.New("unsupported url protocol")
}
