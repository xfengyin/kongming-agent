// 快速开始 - 以库方式嵌入 kongming agent
//
// 演示三件事：
//   1. 构造 Agent 并完成单轮/多轮对话（离线 Mock，无需 API Key）；
//   2. 工具调用：注册计算器，问题命中表达式即短路求值；
//   3. 轻量 RAG：加载本地知识库，检索相关段落拼入上下文。
//
// 运行：go run ./examples/quickstart/main.go

package main

import (
	"context"
	"fmt"

	"github.com/zhuge/kongming/pkg/agent"
	"github.com/zhuge/kongming/pkg/knowledge"
	"github.com/zhuge/kongming/pkg/llm"
	"github.com/zhuge/kongming/pkg/tools"
)

func main() {
	ctx := context.Background()

	// 1. 构造 Agent：Mock Provider 离线演示（接真实 LLM 只需换 Provider）
	a := agent.New(agent.Options{
		Provider:     &llm.MockProvider{},
		SystemPrompt: agent.DefaultSystemPrompt,
		Tools:        tools.NewRegistry(tools.NewCalculator()),
	})

	// 2. 单轮对话
	fmt.Println("=== 单轮对话 ===")
	reply, err := a.Ask(ctx, "天下大势如何？")
	if err != nil {
		panic(err)
	}
	fmt.Printf("🧠 诸葛亮：%s\n", reply.Answer)

	// 3. 多轮对话：历史自动透传（第二轮消息含第一轮问答）
	fmt.Println("\n=== 多轮对话 ===")
	reply, err = a.Ask(ctx, "那我军当如何部署？")
	if err != nil {
		panic(err)
	}
	fmt.Printf("🧠 诸葛亮：%s\n", reply.Answer)
	fmt.Printf("（本轮共 %d 条消息，含历史）\n", reply.Turns)

	// 4. 工具调用：命中"计算"表达式即短路，不调 LLM
	fmt.Println("\n=== 工具调用 ===")
	reply, err = a.Ask(ctx, "计算 123*456")
	if err != nil {
		panic(err)
	}
	fmt.Printf("🛠️  [%s] %s\n", reply.ToolUsed, reply.Answer)

	// 5. 轻量 RAG：加载本地知识库（./knowledge），检索命中段落拼入上下文
	fmt.Println("\n=== 轻量 RAG ===")
	kb, err := knowledge.Load("./knowledge")
	if err != nil {
		fmt.Printf("（跳过：知识库加载失败 %v）\n", err)
		return
	}
	rag := agent.New(agent.Options{
		Provider:     &llm.MockProvider{},
		SystemPrompt: agent.DefaultSystemPrompt,
		Knowledge:    kb,
	})
	reply, err = rag.Ask(ctx, "空城计退敌的故事")
	if err != nil {
		panic(err)
	}
	fmt.Printf("📚 参考知识：%v\n", reply.Knowledge)
	fmt.Printf("🧠 诸葛亮：%s\n", reply.Answer)
}
