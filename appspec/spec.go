package appspec

import (
	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/yaml.v2"
)

var Version = "0.0"

type AppSpec struct {
	Version   *string                `yaml:"version"`
	Resources []map[string]*Resource `yaml:"Resources"`
	Hooks     []*Hook                `yaml:"Hooks,omitempty"`
}

func New(funcName, aliasName, currentVersion, targetVersion string) *AppSpec {
	return &AppSpec{
		Version: aws.String("0.0"),
		Resources: []map[string]*Resource{
			{
				funcName: {
					Type: aws.String("AWS::Lambda::Function"),
					Properties: &Properties{
						Name:           aws.String(funcName),
						Alias:          aws.String(aliasName),
						CurrentVersion: aws.String(currentVersion),
						TargetVersion:  aws.String(targetVersion),
					},
				},
			},
		},
	}
}

func (a *AppSpec) String() string {
	b, _ := yaml.Marshal(a)
	return string(b)
}

type Resource struct {
	Type       *string     `yaml:"Type"`
	Properties *Properties `yaml:"Properties"`
}

type Properties struct {
	Name           *string `yaml:"Name"`
	Alias          *string `yaml:"Alias"`
	CurrentVersion *string `yaml:"CurrentVersion"`
	TargetVersion  *string `yaml:"TargetVersion"`
}

type Hook struct {
	BeforeAllowTraffic string `yaml:"BeforeAllowTraffic,omitempty"`
	AfterAllowTraffic  string `yaml:"AfterAllowTraffic,omitempty"`
}
