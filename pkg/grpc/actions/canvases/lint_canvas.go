package canvases

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/superplanehq/superplane/pkg/database"
	"github.com/superplanehq/superplane/pkg/linter"
	"github.com/superplanehq/superplane/pkg/models"
	pb "github.com/superplanehq/superplane/pkg/protos/canvases"
	"github.com/superplanehq/superplane/pkg/registry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func LintCanvas(
	_ context.Context,
	reg *registry.Registry,
	organizationID string,
	canvasID string,
) (*pb.LintCanvasResponse, error) {
	id, err := uuid.Parse(canvasID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid canvas id: %v", err)
	}

	orgID, err := uuid.Parse(organizationID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid organization id: %v", err)
	}

	_, err = models.FindCanvas(orgID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Errorf(codes.NotFound, "canvas not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to find canvas: %v", err)
	}

	nodes, edges, err := models.FindLiveCanvasSpecInTransaction(database.Conn(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Canvas has no live version yet — nothing to lint.
			return cleanResponse(), nil
		}
		return nil, status.Errorf(codes.Internal, "failed to load canvas spec: %v", err)
	}

	result := linter.LintCanvas(nodes, edges, reg)
	return toProto(result), nil
}

func cleanResponse() *pb.LintCanvasResponse {
	return &pb.LintCanvasResponse{
		Status:   "pass",
		Errors:   []*pb.LintIssue{},
		Warnings: []*pb.LintIssue{},
		Info:     []*pb.LintIssue{},
		Summary:  &pb.LintCanvasSummary{},
	}
}

func toProto(result *linter.LintResult) *pb.LintCanvasResponse {
	return &pb.LintCanvasResponse{
		Status:   result.Status,
		Errors:   issuesToProto(result.Errors),
		Warnings: issuesToProto(result.Warnings),
		Info:     issuesToProto(result.Info),
		Summary: &pb.LintCanvasSummary{
			Total:    int32(result.Summary.Total),
			Errors:   int32(result.Summary.Errors),
			Warnings: int32(result.Summary.Warnings),
			Info:     int32(result.Summary.Info),
		},
	}
}

func issuesToProto(issues []linter.Issue) []*pb.LintIssue {
	out := make([]*pb.LintIssue, 0, len(issues))
	for _, i := range issues {
		out = append(out, &pb.LintIssue{
			Severity: string(i.Severity),
			Rule:     i.Rule,
			NodeId:   i.NodeID,
			NodeName: i.NodeName,
			Message:  i.Message,
		})
	}
	return out
}
