package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.flipt.io/flipt/rpc/flipt"
)

func TestFlag(t *testing.T) {
	f := &flipt.Flag{
		Key:            "flipt",
		Name:           "flipt",
		NamespaceKey:   "flipt",
		Enabled:        false,
		DefaultVariant: nil,
	}
	nf := NewFlag(f)

	assert.Equal(t, nf.Enabled, f.Enabled)
	assert.Equal(t, nf.Key, f.Key)
	assert.Equal(t, nf.Name, f.Name)
	assert.Equal(t, nf.NamespaceKey, f.NamespaceKey)
}

func TestFlagWithDefaultVariant(t *testing.T) {
	f := &flipt.Flag{
		Key:          "flipt",
		Name:         "flipt",
		NamespaceKey: "flipt",
		Enabled:      false,
		DefaultVariant: &flipt.Variant{
			Key: "default-variant",
		},
	}
	nf := NewFlag(f)

	assert.Equal(t, nf.Enabled, f.Enabled)
	assert.Equal(t, nf.Key, f.Key)
	assert.Equal(t, nf.Name, f.Name)
	assert.Equal(t, nf.NamespaceKey, f.NamespaceKey)
	assert.Equal(t, "default-variant", nf.DefaultVariant)
}

func TestVariant(t *testing.T) {
	v := &flipt.Variant{
		Id:      "this-is-an-id",
		FlagKey: "flipt",
		Key:     "flipt",
		Name:    "flipt",
	}

	nv := NewVariant(v)
	assert.Equal(t, nv.Id, v.Id)
	assert.Equal(t, nv.FlagKey, v.FlagKey)
	assert.Equal(t, nv.Key, v.Key)
	assert.Equal(t, nv.Name, v.Name)
}

func testConstraintHelper(t *testing.T, c *flipt.Constraint) {
	t.Helper()
	nc := NewConstraint(c)
	assert.Equal(t, nc.Id, c.Id)
	assert.Equal(t, nc.SegmentKey, c.SegmentKey)
	assert.Equal(t, nc.Type, c.Type.String())
	assert.Equal(t, nc.Property, c.Property)
	assert.Equal(t, nc.Operator, c.Operator)
	assert.Equal(t, nc.Value, c.Value)
}

func TestConstraint(t *testing.T) {
	c := &flipt.Constraint{
		Id:         "this-is-an-id",
		SegmentKey: "flipt",
		Type:       flipt.ComparisonType_STRING_COMPARISON_TYPE,
		Property:   "string",
		Operator:   "eq",
		Value:      "flipt",
	}

	testConstraintHelper(t, c)
}

func TestNamespace(t *testing.T) {
	n := &flipt.Namespace{
		Key:         "flipt",
		Name:        "flipt",
		Description: "flipt",
		Protected:   true,
	}

	nn := NewNamespace(n)
	assert.Equal(t, nn.Key, n.Key)
	assert.Equal(t, nn.Name, n.Name)
	assert.Equal(t, nn.Description, n.Description)
	assert.Equal(t, nn.Protected, n.Protected)
}

func testDistributionHelper(t *testing.T, d *flipt.Distribution) {
	t.Helper()
	nd := NewDistribution(d)
	assert.Equal(t, nd.Id, d.Id)
	assert.Equal(t, nd.RuleId, d.RuleId)
	assert.Equal(t, nd.VariantId, d.VariantId)
	assert.InDelta(t, nd.Rollout, d.Rollout, 0)
}

func TestDistribution(t *testing.T) {
	d := &flipt.Distribution{
		Id:        "this-is-an-id",
		RuleId:    "this-is-a-rule-id",
		VariantId: "this-is-a-variant-id",
		Rollout:   20,
	}

	testDistributionHelper(t, d)
}

func TestSegment(t *testing.T) {
	s := &flipt.Segment{
		Key:         "flipt",
		Name:        "flipt",
		Description: "flipt",
		Constraints: []*flipt.Constraint{
			{
				Id:         "this-is-an-id",
				SegmentKey: "flipt",
				Type:       flipt.ComparisonType_STRING_COMPARISON_TYPE,
				Property:   "string",
				Operator:   "eq",
				Value:      "flipt",
			},
		},
		MatchType:    flipt.MatchType_ANY_MATCH_TYPE,
		NamespaceKey: "flipt",
	}

	ns := NewSegment(s)
	assert.Equal(t, ns.Key, s.Key)
	assert.Equal(t, ns.Name, s.Name)
	assert.Equal(t, ns.Description, s.Description)
	assert.Equal(t, ns.MatchType, s.MatchType.String())
	assert.Equal(t, ns.NamespaceKey, s.NamespaceKey)

	for _, c := range s.Constraints {
		testConstraintHelper(t, c)
	}
}

func TestRule(t *testing.T) {
	t.Run("single segment", func(t *testing.T) {
		r := &flipt.Rule{
			Id:         "this-is-an-id",
			FlagKey:    "flipt",
			SegmentKey: "flipt",
			Rank:       1,
			Distributions: []*flipt.Distribution{
				{
					Id:        "this-is-an-id",
					RuleId:    "this-is-a-rule-id",
					VariantId: "this-is-a-variant-id",
					Rollout:   20,
				},
			},
			NamespaceKey: "flipt",
		}

		nr := NewRule(r)
		assert.Equal(t, nr.Rank, r.Rank)
		assert.Equal(t, nr.Id, r.Id)
		assert.Equal(t, nr.FlagKey, r.FlagKey)
		assert.Equal(t, nr.SegmentKey, r.SegmentKey)

		for _, d := range r.Distributions {
			testDistributionHelper(t, d)
		}
	})
	t.Run("multi segments", func(t *testing.T) {
		r := &flipt.Rule{
			Id:              "this-is-an-id",
			FlagKey:         "flipt",
			SegmentKeys:     []string{"flipt", "io"},
			SegmentOperator: flipt.SegmentOperator_AND_SEGMENT_OPERATOR,
			Rank:            1,
			Distributions: []*flipt.Distribution{
				{
					Id:        "this-is-an-id",
					RuleId:    "this-is-a-rule-id",
					VariantId: "this-is-a-variant-id",
					Rollout:   20,
				},
			},
			NamespaceKey: "flipt",
		}

		nr := NewRule(r)
		assert.Equal(t, r.Rank, nr.Rank)
		assert.Equal(t, r.Id, nr.Id)
		assert.Equal(t, r.FlagKey, nr.FlagKey)
		assert.Equal(t, "flipt,io", nr.SegmentKey)
		assert.Equal(t, "AND_SEGMENT_OPERATOR", nr.SegmentOperator)

		for _, d := range r.Distributions {
			testDistributionHelper(t, d)
		}
	})
}

func TestRollout(t *testing.T) {
	t.Run("single segment", func(t *testing.T) {
		r := &flipt.Rollout{
			Id:           "this-is-an-id",
			FlagKey:      "flipt",
			Rank:         1,
			NamespaceKey: "flipt",
			Rule: &flipt.Rollout_Segment{
				Segment: &flipt.RolloutSegment{
					SegmentKey: "flipt",
					Value:      true,
				},
			},
		}

		nr := NewRollout(r)
		assert.Equal(t, r.Rank, nr.Rank)
		assert.Equal(t, r.FlagKey, nr.FlagKey)
		assert.Equal(t, "flipt", nr.Segment.Key)
		assert.True(t, nr.Segment.Value)
	})

	t.Run("multi segments", func(t *testing.T) {
		r := &flipt.Rollout{
			Id:           "this-is-an-id",
			FlagKey:      "flipt",
			Rank:         1,
			NamespaceKey: "flipt",
			Rule: &flipt.Rollout_Segment{
				Segment: &flipt.RolloutSegment{
					SegmentKeys:     []string{"flipt", "io"},
					Value:           true,
					SegmentOperator: flipt.SegmentOperator_AND_SEGMENT_OPERATOR,
				},
			},
		}

		nr := NewRollout(r)
		assert.Equal(t, r.Rank, nr.Rank)
		assert.Equal(t, r.FlagKey, nr.FlagKey)
		assert.Equal(t, "flipt,io", nr.Segment.Key)
		assert.True(t, nr.Segment.Value)
		assert.Equal(t, "AND_SEGMENT_OPERATOR", nr.Segment.Operator)
	})
}
