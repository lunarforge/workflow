package workflow

// Well-known metadata keys used by FlowWatch and other monitoring tools.
const (
	// MetaKeySubsystem identifies the business domain grouping for this workflow run
	// (e.g., "order-fulfillment", "onboarding", "billing").
	MetaKeySubsystem = "workflow.subsystem"

	// MetaKeyEnvironment identifies the deployment environment (e.g., "production", "staging").
	MetaKeyEnvironment = "workflow.environment"
)

// Subsystem returns the subsystem value from the record's metadata, or an empty string if not set.
func (r *Record) Subsystem() string {
	if r.Metadata == nil {
		return ""
	}
	return r.Metadata[MetaKeySubsystem]
}

// mergeMetadata returns a new map with base values overridden by overrides.
// Neither input is modified.
func mergeMetadata(base, overrides map[string]string) map[string]string {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}
