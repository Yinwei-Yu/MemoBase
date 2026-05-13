package infra

import (
	"context"
	"io"

	pb "memobase/backend/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AgentClient wraps the gRPC connection to the Python agent service.
type AgentClient struct {
	conn          *grpc.ClientConn
	client        pb.AgentServiceClient
	textProcessor pb.TextProcessorServiceClient
}

// NewAgentClient creates a gRPC client connected to the agent service at addr.
func NewAgentClient(addr string) (*AgentClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	return &AgentClient{
		conn:          conn,
		client:        pb.NewAgentServiceClient(conn),
		textProcessor: pb.NewTextProcessorServiceClient(conn),
	}, nil
}

// ChatCompletion sends a unary chat request to the agent service.
func (ac *AgentClient) ChatCompletion(ctx context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	return ac.client.ChatCompletion(ctx, req)
}

// ChatCompletionStream opens a server-streaming chat request.
func (ac *AgentClient) ChatCompletionStream(ctx context.Context, req *pb.ChatRequest) (pb.AgentService_ChatCompletionStreamClient, error) {
	return ac.client.ChatCompletionStream(ctx, req)
}

// StreamEvent wraps a single ChatEvent from the gRPC stream.
type StreamEvent struct {
	Event *pb.ChatEvent
	Err   error
}

// RecvEvents reads all events from a gRPC stream and sends them to a channel.
func RecvEvents(stream pb.AgentService_ChatCompletionStreamClient) <-chan StreamEvent {
	ch := make(chan StreamEvent, 16)
	go func() {
		defer close(ch)
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				return
			}
			ch <- StreamEvent{Event: event, Err: err}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

// Close closes the underlying gRPC connection.
func (ac *AgentClient) Close() error {
	return ac.conn.Close()
}

// HealthCheck checks the agent service health via gRPC.
func (ac *AgentClient) HealthCheck(ctx context.Context) (*pb.HealthResponse, error) {
	return ac.client.HealthCheck(ctx, &pb.HealthRequest{})
}

// Tokenize splits text into tokens via the Python text processor service.
func (ac *AgentClient) Tokenize(ctx context.Context, text string) ([]string, error) {
	resp, err := ac.textProcessor.Tokenize(ctx, &pb.TokenizeRequest{Text: text})
	if err != nil {
		return nil, err
	}
	return resp.Tokens, nil
}

// ChunkDocument splits a document into chunks via the Python text processor service.
func (ac *AgentClient) ChunkDocument(ctx context.Context, content string, maxChunkSize, overlap int, markdownAware bool) ([]string, error) {
	mdAware := markdownAware
	resp, err := ac.textProcessor.ChunkDocument(ctx, &pb.ChunkRequest{
		Content:       content,
		MaxChunkSize:  int32(maxChunkSize),
		Overlap:       int32(overlap),
		MarkdownAware: &mdAware,
	})
	if err != nil {
		return nil, err
	}
	return resp.Chunks, nil
}

// RetrieveChunks performs hybrid retrieval (BM25 + vector + RRF) via the Python text processor service.
func (ac *AgentClient) RetrieveChunks(ctx context.Context, kbID, query string, topK int) ([]*pb.RetrievedChunk, bool, error) {
	resp, err := ac.textProcessor.RetrieveChunks(ctx, &pb.RetrieveChunksRequest{
		KbId:  kbID,
		Query: query,
		TopK:  int32(topK),
	})
	if err != nil {
		return nil, false, err
	}
	return resp.Chunks, resp.Degraded, nil
}

// InvalidateCache clears the BM25 cache for a knowledge base via the Python text processor service.
func (ac *AgentClient) InvalidateCache(ctx context.Context, kbID string) error {
	_, err := ac.textProcessor.InvalidateCache(ctx, &pb.InvalidateCacheRequest{KbId: kbID})
	return err
}
