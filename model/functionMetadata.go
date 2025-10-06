package model

import (
	"encoding/json"
	"strconv"
)

type FunctionMetadata struct {
	Name                  string
	Version               string
	TemplateNames         []string
	DeployedOn            []string
	StrongCollWriteAccess bool
	IsAuth                bool
}

func NewFunctionMetadata(metadata map[string]string) (*FunctionMetadata, error) {
	var templateNames []string
	err := json.Unmarshal([]byte(metadata["Template-Names"]), &templateNames)
	if err != nil {
		return nil, err
	}

	var deployedOn []string
	err = json.Unmarshal([]byte(metadata["Deployed-On"]), &deployedOn)
	if err != nil {
		return nil, err
	}

	strongCollAccess, err := strconv.ParseBool(metadata["Strong-Coll-Write-Access"])
	if err != nil {
		return nil, err
	}

	isAuth, err := strconv.ParseBool(metadata["Is-Auth"])
	return &FunctionMetadata{
		Name:                  metadata["Name"],
		Version:               metadata["Version"],
		TemplateNames:         templateNames,
		DeployedOn:            deployedOn,
		StrongCollWriteAccess: strongCollAccess,
		IsAuth:                isAuth,
	}, err
}

func (f *FunctionMetadata) ToStringMap() (map[string]string, error) {
	templateNamesBytes, err := json.Marshal(f.TemplateNames)
	if err != nil {
		return nil, err
	}
	deployedOnBytes, err := json.Marshal(f.DeployedOn)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"Name":                     f.Name,
		"Version":                  f.Version,
		"Template-Names":           string(templateNamesBytes),
		"Deployed-On":              string(deployedOnBytes),
		"Strong-Coll-Write-Access": strconv.FormatBool(f.StrongCollWriteAccess),
		"Is-Auth":                  strconv.FormatBool(f.IsAuth),
	}, nil
}
