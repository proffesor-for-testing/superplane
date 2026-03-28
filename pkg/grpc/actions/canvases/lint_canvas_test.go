package canvases

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superplanehq/superplane/pkg/database"
	"github.com/superplanehq/superplane/pkg/models"
	"gorm.io/datatypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/superplanehq/superplane/test/support"
)

func Test__LintCanvas(t *testing.T) {
	r := support.Setup(t)

	t.Run("canvas does not exist -> error", func(t *testing.T) {
		_, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), uuid.New().String())
		s, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.NotFound, s.Code())
	})

	t.Run("invalid canvas id -> error", func(t *testing.T) {
		_, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), "invalid-id")
		s, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, s.Code())
	})

	t.Run("invalid organization id -> error", func(t *testing.T) {
		_, err := LintCanvas(context.Background(), r.Registry, "invalid-org", uuid.New().String())
		s, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, s.Code())
	})

	t.Run("canvas with no live version -> passes", func(t *testing.T) {
		// Create canvas without live version
		canvas := &models.Canvas{
			ID:             uuid.New(),
			OrganizationID: r.Organization.ID,
			Name:           "no-live-version",
			CreatedBy:      &r.User,
		}
		require.NoError(t, database.Conn().Create(canvas).Error)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "pass", response.Status)
		assert.Empty(t, response.Errors)
		assert.Empty(t, response.Warnings)
		assert.Empty(t, response.Info)
	})

	t.Run("valid canvas with trigger and connected nodes -> passes", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
				{
					NodeID: "component-1",
					Name:   "Process",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "noop"},
					}),
				},
			},
			[]models.Edge{
				{SourceID: "trigger-1", TargetID: "component-1", Channel: "default"},
			},
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "pass", response.Status)
	})

	t.Run("canvas with orphan node -> fails", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
				{
					NodeID: "orphan-1",
					Name:   "Orphan",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "noop"},
					}),
				},
			},
			[]models.Edge{}, // No edges - orphan node
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "fail", response.Status)
		assert.GreaterOrEqual(t, response.Summary.Errors, int32(1))

		orphanErrors := 0
		for _, e := range response.Errors {
			if e.Rule == "orphan-node" && e.NodeId == "orphan-1" {
				orphanErrors++
			}
		}
		assert.Equal(t, 1, orphanErrors)
	})

	t.Run("canvas with missing approval gate -> warning", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
				{
					NodeID: "delete-1",
					Name:   "Delete Release",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "github.deleteRelease"},
					}),
				},
			},
			[]models.Edge{
				{SourceID: "trigger-1", TargetID: "delete-1", Channel: "default"},
			},
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		// Warning doesn't cause failure
		assert.Equal(t, "pass", response.Status)
		assert.GreaterOrEqual(t, response.Summary.Warnings, int32(1))

		approvalWarnings := 0
		for _, w := range response.Warnings {
			if w.Rule == "missing-approval-gate" {
				approvalWarnings++
			}
		}
		assert.Equal(t, 1, approvalWarnings)
	})

	t.Run("canvas with cycle -> fails", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
				{
					NodeID: "node-a",
					Name:   "Node A",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "noop"},
					}),
				},
				{
					NodeID: "node-b",
					Name:   "Node B",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "noop"},
					}),
				},
			},
			[]models.Edge{
				{SourceID: "trigger-1", TargetID: "node-a", Channel: "default"},
				{SourceID: "node-a", TargetID: "node-b", Channel: "default"},
				{SourceID: "node-b", TargetID: "node-a", Channel: "default"}, // Creates cycle
			},
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "fail", response.Status)

		cycleErrors := 0
		for _, e := range response.Errors {
			if e.Rule == "cycle-detected" {
				cycleErrors++
			}
		}
		assert.GreaterOrEqual(t, cycleErrors, 2) // Both nodes in cycle
	})

	t.Run("canvas with empty expression -> fails", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
				{
					NodeID: "if-1",
					Name:   "Check",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "if"},
					}),
					Configuration: datatypes.NewJSONType(map[string]any{"expression": ""}), // Empty!
				},
			},
			[]models.Edge{
				{SourceID: "trigger-1", TargetID: "if-1", Channel: "default"},
			},
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "fail", response.Status)

		exprErrors := 0
		for _, e := range response.Errors {
			if e.Rule == "empty-expression-field" && e.NodeId == "if-1" {
				exprErrors++
			}
		}
		assert.Equal(t, 1, exprErrors)
	})

	t.Run("canvas with multiple issues -> reports all", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
				{
					NodeID: "orphan-1",
					Name:   "Orphan",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "noop"},
					}),
				},
				{
					NodeID: "delete-1",
					Name:   "Delete",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "github.deleteRelease"},
					}),
				},
			},
			[]models.Edge{
				{SourceID: "trigger-1", TargetID: "delete-1", Channel: "default"},
				// orphan-1 not connected
			},
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		assert.Equal(t, "fail", response.Status)
		assert.GreaterOrEqual(t, response.Summary.Total, int32(2))
		assert.GreaterOrEqual(t, response.Summary.Errors, int32(1))  // orphan
		assert.GreaterOrEqual(t, response.Summary.Warnings, int32(1)) // missing approval
	})

	t.Run("response structure is correct", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
			},
			[]models.Edge{},
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		require.NotNil(t, response)

		// Verify response structure
		assert.NotEmpty(t, response.Status)
		assert.NotNil(t, response.Errors)
		assert.NotNil(t, response.Warnings)
		assert.NotNil(t, response.Info)
		assert.NotNil(t, response.Summary)

		// Verify summary structure
		assert.GreaterOrEqual(t, response.Summary.Total, int32(0))
		assert.GreaterOrEqual(t, response.Summary.Errors, int32(0))
		assert.GreaterOrEqual(t, response.Summary.Warnings, int32(0))
		assert.GreaterOrEqual(t, response.Summary.Info, int32(0))

		// Verify consistency
		assert.Equal(t, response.Summary.Total, response.Summary.Errors+response.Summary.Warnings+response.Summary.Info)
		assert.Equal(t, int32(len(response.Errors)), response.Summary.Errors)
		assert.Equal(t, int32(len(response.Warnings)), response.Summary.Warnings)
		assert.Equal(t, int32(len(response.Info)), response.Summary.Info)
	})

	t.Run("lint issue structure is correct", func(t *testing.T) {
		canvas, _ := support.CreateCanvas(
			t,
			r.Organization.ID,
			r.User,
			[]models.CanvasNode{
				{
					NodeID: "trigger-1",
					Name:   "Trigger",
					Type:   models.NodeTypeTrigger,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Trigger: &models.TriggerRef{Name: "manual"},
					}),
				},
				{
					NodeID: "orphan-1",
					Name:   "Orphan Node",
					Type:   models.NodeTypeComponent,
					Ref: datatypes.NewJSONType(models.NodeRef{
						Component: &models.ComponentRef{Name: "noop"},
					}),
				},
			},
			[]models.Edge{}, // Orphan node
		)

		response, err := LintCanvas(context.Background(), r.Registry, r.Organization.ID.String(), canvas.ID.String())
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(response.Errors), 1)

		issue := response.Errors[0]
		assert.NotEmpty(t, issue.Severity)
		assert.NotEmpty(t, issue.Rule)
		assert.NotEmpty(t, issue.NodeId)
		assert.NotEmpty(t, issue.NodeName)
		assert.NotEmpty(t, issue.Message)
		assert.Equal(t, "error", issue.Severity)
		assert.Equal(t, "orphan-1", issue.NodeId)
		assert.Equal(t, "Orphan Node", issue.NodeName)
	})
}
