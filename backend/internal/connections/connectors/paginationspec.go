package connectors

import (
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/httpclient"
)

func buildRESTPaginator(spec *connections.PaginationSpec) (httpclient.Paginator, error) {
	switch spec.Strategy {
	case "offset":
		return &httpclient.OffsetLimitPaginator{OffsetParam: spec.OffsetParam, LimitParam: spec.LimitParam, PageSize: spec.PageSize, ItemsPath: spec.ItemsPath}, nil
	case "page":
		return &httpclient.PagePaginator{PageParam: spec.PageParam, PageSizeParam: spec.PageSizeParam, PageSize: spec.PageSize, ItemsPath: spec.ItemsPath}, nil
	case "cursor":
		return &httpclient.CursorPaginator{CursorParam: spec.CursorParam, CursorPath: spec.CursorPath}, nil
	case "linkHeader":
		return &httpclient.LinkHeaderPaginator{}, nil
	default:
		return nil, fmt.Errorf("unsupported pagination strategy %q for a REST connection", spec.Strategy)
	}
}

func buildGraphQLPaginator(dataPath string, spec *connections.PaginationSpec) (httpclient.Paginator, error) {
	if spec.Strategy != "graphqlRelay" {
		return nil, fmt.Errorf("unsupported pagination strategy %q for a GraphQL connection - use \"graphqlRelay\"", spec.Strategy)
	}
	return &httpclient.GraphQLRelayPaginator{DataPath: dataPath, CursorVariable: spec.GraphQLCursorVariable, PageSizeVariable: spec.GraphQLPageSizeVariable, PageSize: spec.PageSize}, nil
}

func maxPagesOf(spec *connections.PaginationSpec, configured int) int {
	if spec != nil && spec.MaxPages > 0 {
		return spec.MaxPages
	}
	return configured
}
