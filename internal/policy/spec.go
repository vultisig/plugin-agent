package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kaptinlin/jsonschema"
	"google.golang.org/protobuf/encoding/protojson"

	rtypes "github.com/vultisig/recipes/types"
	"github.com/vultisig/verifier/plugin"
	"github.com/vultisig/verifier/types"
)

type AgentSpec struct {
	plugin.Unimplemented
	path       string
	cachedSpec *rtypes.RecipeSchema
}

func NewAgentSpec(path string) (*AgentSpec, error) {
	spec := &AgentSpec{
		path: path,
	}

	err := spec.loadSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to load recipe specification: %w", err)
	}

	return spec, nil
}

func (s *AgentSpec) loadSpec() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	var spec rtypes.RecipeSchema
	err = protojson.Unmarshal(data, &spec)
	if err != nil {
		return fmt.Errorf("failed to unmarshal spec: %w", err)
	}

	s.cachedSpec = &spec
	return nil
}

func (s *AgentSpec) GetRecipeSpecification() (*rtypes.RecipeSchema, error) {
	if s.cachedSpec == nil {
		err := s.loadSpec()
		if err != nil {
			return nil, err
		}
	}

	return s.cachedSpec, nil
}

func (s *AgentSpec) Suggest(_ context.Context, cfg map[string]any) (*rtypes.PolicySuggest, error) {
	spec, err := s.GetRecipeSpecification()
	if err != nil {
		return nil, fmt.Errorf("failed to get recipe specification: %w", err)
	}

	err = s.validateConfiguration(cfg, spec)
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	rules, err := s.createRules(cfg, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create rules: %w", err)
	}

	return &rtypes.PolicySuggest{
		Rules: rules,
	}, nil
}

func (s *AgentSpec) validateConfiguration(cfg map[string]any, spec *rtypes.RecipeSchema) error {
	schemaMap := spec.Configuration.AsMap()
	schemaBytes, err := json.Marshal(schemaMap)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaBytes)
	if err != nil {
		return fmt.Errorf("failed to compile JSON schema: %w", err)
	}

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	res := schema.Validate(cfgBytes)
	if !res.IsValid() {
		var errStrs []string
		for _, e := range res.Errors {
			errStrs = append(errStrs, e.Error())
		}
		return fmt.Errorf("configuration validation error: %s", strings.Join(errStrs, ", "))
	}

	return nil
}

func (s *AgentSpec) createRules(cfg map[string]any, spec *rtypes.RecipeSchema) ([]*rtypes.Rule, error) {
	var rules []*rtypes.Rule

	for _, resource := range spec.SupportedResources {
		constraints, err := s.createParameterConstraints(cfg, resource)
		if err != nil {
			return nil, fmt.Errorf("failed to create parameter constraints for %s: %w", resource.ResourcePath.Full, err)
		}

		rule := &rtypes.Rule{
			Resource:             resource.ResourcePath.Full,
			Effect:               rtypes.Effect_EFFECT_ALLOW,
			ParameterConstraints: constraints,
			Target: &rtypes.Target{
				TargetType: resource.Target,
			},
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

func (s *AgentSpec) createParameterConstraints(_ map[string]any, resource *rtypes.ResourcePattern) ([]*rtypes.ParameterConstraint, error) {
	var constraints []*rtypes.ParameterConstraint

	for _, paramCap := range resource.ParameterCapabilities {
		constraint := &rtypes.ParameterConstraint{
			ParameterName: paramCap.ParameterName,
			Constraint: &rtypes.Constraint{
				Type:     paramCap.SupportedTypes,
				Required: paramCap.Required,
			},
		}

		constraints = append(constraints, constraint)
	}

	return constraints, nil
}

func (s *AgentSpec) ValidatePluginPolicy(pol types.PluginPolicy) error {
	spec, err := s.GetRecipeSpecification()
	if err != nil {
		return fmt.Errorf("failed to get recipe spec: %w", err)
	}

	return plugin.ValidatePluginPolicy(pol, spec)
}
