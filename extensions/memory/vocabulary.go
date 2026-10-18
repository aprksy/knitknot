package memory

const (
	LabelUserProfile    = "user_profile"
	LabelSessionSummary = "session_summary"
	LabelPRTurn         = "pr_turn"
	LabelTopicCluster   = "topic_cluster"
	LabelTag            = "tag"
)

const (
	EdgeChildOf    = "child-of"
	EdgeSimilarTo  = "similar-to"
	EdgeSupersedes = "supersedes"
	EdgeTagged     = "tagged"
)

const (
	PropMemoryID       = "memory_id"
	PropUserID         = "user_id"
	PropPayload        = "payload"
	PropPayloadRef     = "payload_ref"
	PropFormat         = "format"
	PropEmbedding      = "embedding"
	PropEmbeddingRef   = "embedding_ref"
	PropEmbeddingModel = "embedding_model"
	PropEmbeddingDim   = "embedding_dim"
	PropCreatedAt      = "created_at"
	PropTags           = "tags"
	PropVersion        = "version"
	PropIsStale        = "is_stale"
	PropSupersededBy   = "superseded_by"
	PropScore          = "score"
	PropModel          = "model"
	PropObservedAt     = "observed_at"
	PropName           = "name"
)

// SupportedLabels lists every node label the memory extension recognizes.
var SupportedLabels = []string{
	LabelUserProfile,
	LabelSessionSummary,
	LabelPRTurn,
	LabelTopicCluster,
	LabelTag,
}

// SupportedEdgeKinds lists every edge kind the memory extension recognizes.
var SupportedEdgeKinds = []string{
	EdgeChildOf,
	EdgeSimilarTo,
	EdgeSupersedes,
	EdgeTagged,
}

var supportedLabelSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SupportedLabels))
	for _, l := range SupportedLabels {
		m[l] = struct{}{}
	}
	return m
}()

var supportedEdgeKindSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SupportedEdgeKinds))
	for _, k := range SupportedEdgeKinds {
		m[k] = struct{}{}
	}
	return m
}()

// IsSupportedLabel reports whether s is a recognized memory node label.
func IsSupportedLabel(s string) bool {
	_, ok := supportedLabelSet[s]
	return ok
}

// IsSupportedEdgeKind reports whether s is a recognized memory edge kind.
func IsSupportedEdgeKind(s string) bool {
	_, ok := supportedEdgeKindSet[s]
	return ok
}
