package service_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
	"github.com/zhuge/kongming/pkg/transport/grpc/interceptor"
	svc "github.com/zhuge/kongming/pkg/transport/grpc/service"
)

// mockCommander 桩实现 port.Commander，仅在测试中注入。
type mockCommander struct {
	dispatchResp *model.BattleReport
	dispatchErr  error
	orderResp    *model.Order
	orderErr     error
	listResp     []*model.Order
	listErr      error
}

func (m *mockCommander) Dispatch(_ context.Context, _ *model.Order) (*model.BattleReport, error) {
	return m.dispatchResp, m.dispatchErr
}

func (m *mockCommander) PlanStrategy(_ context.Context, _ *model.Order) (*model.Strategy, error) {
	return nil, nil
}

func (m *mockCommander) Review(_ context.Context, _ *model.BattleReport) error {
	return nil
}

func (m *mockCommander) GetOrder(_ context.Context, _ model.OrderID) (*model.Order, error) {
	return m.orderResp, m.orderErr
}

func (m *mockCommander) ListOrders(_ context.Context, _ model.State) ([]*model.Order, error) {
	return m.listResp, m.listErr
}

// mockPool 桩实现 port.GeneralPool（最小集，仅覆盖 ListGenerals）。
type mockPool struct {
	listResp []*model.General
	listErr  error
}

func (m *mockPool) List(_ context.Context) ([]*model.General, error) { return m.listResp, m.listErr }
func (m *mockPool) Get(_ context.Context, _ model.GeneralID) (*model.General, error) {
	return nil, nil
}
func (m *mockPool) Register(_ context.Context, _ *model.General) error    { return nil }
func (m *mockPool) Unregister(_ context.Context, _ model.GeneralID) error { return nil }
func (m *mockPool) SelectBest(_ string) (*model.General, error)           { return nil, nil }
func (m *mockPool) Execute(_ context.Context, _ model.GeneralID, _ *model.Order) (*model.GeneralReport, error) {
	return nil, nil
}

// mockVault 桩实现 port.Vault（最小集，覆盖 ListJinnang / ExecuteJinnang）。
type mockVault struct {
	listResp   []*model.Jinnang
	listErr    error
	execResp   *model.JinnangOutput
	execErr    error
	execCalled bool
}

func (m *mockVault) RegisterSkill(_ *model.Jinnang, _ model.JinnangHandler) error { return nil }
func (m *mockVault) GetJinnang(_ string) (*model.Jinnang, error)                  { return nil, nil }
func (m *mockVault) ListJinnang() ([]*model.Jinnang, error)                       { return m.listResp, m.listErr }
func (m *mockVault) Execute(_ context.Context, _ string, _ model.JinnangInput) (*model.JinnangOutput, error) {
	m.execCalled = true
	return m.execResp, m.execErr
}
func (m *mockVault) LoadFromDir(_ context.Context, _ string) error { return nil }

// mockEngine 桩实现 port.Engine（最小集，覆盖 RunWorkflow）。
type mockEngine struct {
	execResp *model.ExecutionContext
	execErr  error
}

func (m *mockEngine) RegisterWorkflow(_ *model.Workflow) error      { return nil }
func (m *mockEngine) GetWorkflow(_ string) (*model.Workflow, error) { return nil, nil }
func (m *mockEngine) Execute(_ context.Context, _ string, _ map[string]any) (*model.ExecutionContext, error) {
	return m.execResp, m.execErr
}
func (m *mockEngine) RegisterNodeExecutor(_ model.NodeType, _ port.NodeExecutor) {}

// portNodeExecutor 是 port.NodeExecutor 的本地别名（避免在测试中再 import 一遍）。
type portNodeExecutor = port.NodeExecutor

// newBufconnServer 启动一个真实 gRPC server，监听 bufconn。
//
// 与 plan 保持一致：bufconn 拉起 server.Serve、client 通过 grpc.WithContextDialer
// 走内存通道；不需要 TCP 端口，便于 CI 跑通。
func newBufconnServer(t *testing.T, register func(s *grpc.Server)) (*grpc.ClientConn, func()) {
	t.Helper()
	lis := bufconn.Listen(1024 * 64)
	s := grpc.NewServer(
		grpc.UnaryInterceptor(interceptor.Chain(
			interceptor.TraceID(),
			interceptor.Recovery(zap.NewNop()),
		)),
	)
	register(s)
	go func() { _ = s.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return conn, func() {
		_ = conn.Close()
		s.Stop()
		_ = lis.Close()
	}
}

// TestService_Dispatch 成功路径：mock 返回成功 BattleReport，gRPC client 收到 order_id/success=true。
func TestService_Dispatch(t *testing.T) {
	cmder := &mockCommander{
		dispatchResp: &model.BattleReport{
			OrderID:     "ord-1",
			Success:     true,
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.Dispatch(context.Background(), &pb.DispatchRequest{Name: "test", Priority: 2})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.GetOrderId(), "必须返回生成的 order id")
	assert.True(t, resp.GetSuccess())
	assert.NotEmpty(t, resp.GetReportJson(), "BattleReport 应被 JSON 化")
	assert.Contains(t, resp.GetMessage(), "dispatched")
}

// TestService_Dispatch_InvalidArgument 验证：Name 为空时返回 InvalidArgument。
func TestService_Dispatch_InvalidArgument(t *testing.T) {
	cmder := &mockCommander{}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	_, err := cli.Dispatch(context.Background(), &pb.DispatchRequest{Name: "", Priority: 1})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// TestService_Dispatch_PriorityOutOfRange 验证：Priority 越界返 InvalidArgument。
func TestService_Dispatch_PriorityOutOfRange(t *testing.T) {
	cmder := &mockCommander{}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	_, err := cli.Dispatch(context.Background(), &pb.DispatchRequest{Name: "x", Priority: 99})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// TestService_GetOrder_NotFound 验证：mock 返回 NOT_FOUND 错误时，gRPC 返回 codes.NotFound。
func TestService_GetOrder_NotFound(t *testing.T) {
	cmder := &mockCommander{
		orderErr: domerrs.New(domerrs.NOT_FOUND, "order not found"),
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	_, err := cli.GetOrder(context.Background(), &pb.GetOrderRequest{Id: "missing"})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestService_GetOrder_OK 验证：成功时返回字段完整的 Order。
func TestService_GetOrder_OK(t *testing.T) {
	now := time.Now()
	cmder := &mockCommander{
		orderResp: &model.Order{
			ID:        "ord-1",
			Name:      "demo",
			State:     model.StateCompleted,
			Priority:  model.PriorityHigh,
			CreatedAt: now,
		},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.GetOrder(context.Background(), &pb.GetOrderRequest{Id: "ord-1"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "ord-1", resp.GetId())
	assert.Equal(t, "demo", resp.GetName())
	assert.Equal(t, int32(model.StateCompleted), resp.GetState())
	assert.NotNil(t, resp.GetCreatedAt())
}

// TestService_ListOrders_Empty 验证：池空时返回空列表而非 nil/error。
func TestService_ListOrders_Empty(t *testing.T) {
	cmder := &mockCommander{listResp: nil}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.ListOrders(context.Background(), &pb.ListOrdersRequest{StateFilter: 0})
	require.NoError(t, err)
	require.NotNil(t, resp)
	// proto3 wire 格式下，无元素的 repeated 字段会反序列化为 nil slice；
	// 业务上"空"和"nil"等价，这里用 Empty 断言即可。
	assert.Empty(t, resp.GetOrders())
}

// TestService_ListOrders_Populated 验证：返回多订单时正确映射。
func TestService_ListOrders_Populated(t *testing.T) {
	cmder := &mockCommander{
		listResp: []*model.Order{
			{ID: "o1", Name: "first", State: model.StatePending, Priority: model.PriorityNormal},
			{ID: "o2", Name: "second", State: model.StateCompleted, Priority: model.PriorityHigh},
		},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.ListOrders(context.Background(), &pb.ListOrdersRequest{})
	require.NoError(t, err)
	assert.Len(t, resp.GetOrders(), 2)
	assert.Equal(t, "o1", resp.GetOrders()[0].GetId())
	assert.Equal(t, "o2", resp.GetOrders()[1].GetId())
}

// TestService_ListGenerals 验证：将领池返回的 General 正确映射。
func TestService_ListGenerals(t *testing.T) {
	pool := &mockPool{
		listResp: []*model.General{
			{ID: "g1", Name: "关羽", Title: "前将军", Skills: []string{"slash"}, State: int(model.GeneralIdle)},
		},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, nil, pool, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.ListGenerals(context.Background(), &pb.ListGeneralsRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetGenerals(), 1)
	assert.Equal(t, "g1", resp.GetGenerals()[0].GetId())
	assert.Equal(t, "关羽", resp.GetGenerals()[0].GetName())
	assert.Equal(t, []string{"slash"}, resp.GetGenerals()[0].GetSkills())
}

// TestService_ListJinnang 验证：锦囊列表正确序列化（含 type 字段映射）。
func TestService_ListJinnang(t *testing.T) {
	vault := &mockVault{
		listResp: []*model.Jinnang{
			{ID: "fire-attack", Name: "火攻", Type: model.JinnangTool, Version: "1.0.0"},
		},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, nil, nil, vault))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.ListJinnang(context.Background(), &pb.ListJinnangRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetJinnangs(), 1)
	j := resp.GetJinnangs()[0]
	assert.Equal(t, "fire-attack", j.GetId())
	assert.Equal(t, "火攻", j.GetName())
	assert.Equal(t, int32(2), j.GetType()) // tool=2
	assert.Equal(t, "1.0.0", j.GetVersion())
}

// TestService_ExecuteJinnang_OK 验证：成功执行时 Data 透传。
func TestService_ExecuteJinnang_OK(t *testing.T) {
	vault := &mockVault{
		execResp: &model.JinnangOutput{Success: true, Data: []byte("payload")},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, nil, nil, vault))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.ExecuteJinnang(context.Background(), &pb.ExecuteJinnangRequest{
		Id:     "fire-attack",
		Params: map[string]string{"target": "we"},
		Data:   []byte("seed"),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.GetSuccess())
	assert.True(t, vault.execCalled, "vault.Execute 必须被调用")
	assert.Equal(t, []byte("payload"), resp.GetData())
}

// TestService_ExecuteJinnang_MissingID 验证：id 缺失返 InvalidArgument。
func TestService_ExecuteJinnang_MissingID(t *testing.T) {
	vault := &mockVault{}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, nil, nil, vault))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	_, err := cli.ExecuteJinnang(context.Background(), &pb.ExecuteJinnangRequest{Id: ""})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// TestService_ExecuteJinnang_NotFound 验证：mock 返回 NOT_FOUND 时映射到 codes.NotFound。
func TestService_ExecuteJinnang_NotFound(t *testing.T) {
	vault := &mockVault{execErr: domerrs.New(domerrs.NOT_FOUND, "jinnang not found")}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, nil, nil, vault))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	_, err := cli.ExecuteJinnang(context.Background(), &pb.ExecuteJinnangRequest{Id: "ghost"})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

// TestService_RunWorkflow_OK 验证：NodeStates 投影为 map[string]string，整体 success 由节点状态聚合。
func TestService_RunWorkflow_OK(t *testing.T) {
	engine := &mockEngine{
		execResp: &model.ExecutionContext{
			WorkflowID: "wf-1",
			NodeStates: map[string]model.NodeState{
				"n1": {ID: "n1", Status: model.NodeStatusOK},
				"n2": {ID: "n2", Status: model.NodeStatusOK},
			},
		},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, engine, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.RunWorkflow(context.Background(), &pb.RunWorkflowRequest{
		WorkflowId: "wf-1",
		Inputs:     map[string]string{"k": "v"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.GetSuccess())
	assert.Equal(t, "ok", resp.GetNodeStates()["n1"])
	assert.Equal(t, "ok", resp.GetNodeStates()["n2"])
}

// TestService_RunWorkflow_PartialFail 验证：任一节点 failed → success=false。
func TestService_RunWorkflow_PartialFail(t *testing.T) {
	engine := &mockEngine{
		execResp: &model.ExecutionContext{
			NodeStates: map[string]model.NodeState{
				"n1": {ID: "n1", Status: model.NodeStatusOK},
				"n2": {ID: "n2", Status: model.NodeStatusFailed},
			},
		},
	}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, engine, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	resp, err := cli.RunWorkflow(context.Background(), &pb.RunWorkflowRequest{WorkflowId: "wf-1"})
	require.NoError(t, err)
	assert.False(t, resp.GetSuccess())
	assert.Equal(t, "failed", resp.GetNodeStates()["n2"])
}

// TestService_RunWorkflow_InvalidID 验证：workflow_id 缺失返 InvalidArgument。
func TestService_RunWorkflow_InvalidID(t *testing.T) {
	engine := &mockEngine{}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(nil, nil, engine, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	_, err := cli.RunWorkflow(context.Background(), &pb.RunWorkflowRequest{WorkflowId: ""})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

// TestService_Dispatch_UnknownErrorMapsInternal 验证：未知类型错误 → codes.Internal。
func TestService_Dispatch_UnknownErrorMapsInternal(t *testing.T) {
	cmder := &mockCommander{dispatchErr: errors.New("boom")}
	conn, stop := newBufconnServer(t, func(s *grpc.Server) {
		pb.RegisterKongmingServer(s, svc.New(cmder, nil, nil, nil, nil))
	})
	defer stop()

	cli := pb.NewKongmingClient(conn)
	_, err := cli.Dispatch(context.Background(), &pb.DispatchRequest{Name: "x", Priority: 1})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}
