package memory

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/easeaico/adk-memory-agent/internal/config"
	adkmemory "google.golang.org/adk/memory"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// mockStore is a mock implementation of Store for testing
type mockStore struct {
	projectRules      []string
	savedExperiences  []savedExperience
	searchResults     []Experience
	searchError       error
	saveError         error
	projectRulesError error
}

type savedExperience struct {
	pattern, cause, solution string
	vector                   []float32
}

func (m *mockStore) GetProjectRules(ctx context.Context) ([]string, error) {
	if m.projectRulesError != nil {
		return nil, m.projectRulesError
	}
	return m.projectRules, nil
}

func (m *mockStore) SearchSimilarIssues(ctx context.Context, queryVector []float32, limit int) ([]Experience, error) {
	if m.searchError != nil {
		return nil, m.searchError
	}
	return m.searchResults, nil
}

func (m *mockStore) SaveExperience(ctx context.Context, pattern, cause, solution string, vector []float32) error {
	if m.saveError != nil {
		return m.saveError
	}
	m.savedExperiences = append(m.savedExperiences, savedExperience{
		pattern:  pattern,
		cause:    cause,
		solution: solution,
		vector:   vector,
	})
	return nil
}

func (m *mockStore) Close() {
}

// mockEmbedder is a mock implementation of EmbedderInterface for testing
type mockEmbedder struct {
	embedError error
	embedValue []float32
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.embedError != nil {
		return nil, m.embedError
	}
	if m.embedValue != nil {
		return m.embedValue, nil
	}
	// Default: return a simple vector
	return []float32{0.1, 0.2, 0.3}, nil
}

// newTestService creates a Service for testing with a mock embedder
func newTestService(store Store, mockEmbed EmbedderInterface) *Service {
	return &Service{
		store:    store,
		embedder: mockEmbed,
	}
}

// mockSession is a mock implementation of session.Session for testing
type mockSession struct {
	id       string
	appName  string
	userID   string
	events   []*session.Event
	lastTime time.Time
}

func (m *mockSession) ID() string {
	return m.id
}

func (m *mockSession) AppName() string {
	return m.appName
}

func (m *mockSession) UserID() string {
	return m.userID
}

func (m *mockSession) State() session.State {
	return &mockState{}
}

// mockState is a simple implementation of session.State for testing
type mockState struct{}

func (m *mockState) Get(key string) (any, error) {
	return nil, errors.New("key not found")
}

func (m *mockState) Set(key string, value any) error {
	return nil
}

func (m *mockState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		// No items
	}
}

func (m *mockSession) Events() session.Events {
	return &mockEvents{events: m.events}
}

func (m *mockSession) LastUpdateTime() time.Time {
	return m.lastTime
}

// mockEvents is a mock implementation of session.Events
type mockEvents struct {
	events []*session.Event
}

func (m *mockEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, e := range m.events {
			if !yield(e) {
				return
			}
		}
	}
}

func (m *mockEvents) Len() int {
	return len(m.events)
}

func (m *mockEvents) At(i int) *session.Event {
	if i < 0 || i >= len(m.events) {
		return nil
	}
	return m.events[i]
}

func TestService_AddSession(t *testing.T) {
	ctx := context.Background()
	defaultVector := []float32{0.1, 0.2, 0.3}

	tests := []struct {
		name           string
		session        *mockSession
		embedder       *mockEmbedder
		store          *mockStore
		wantSaved      bool
		wantError      bool
		wantErrorMsg   string
		checkSavedData func(*testing.T, []savedExperience)
	}{
		{
			name: "successful save with user query and agent response",
			session: &mockSession{
				id:      "test-session-1",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "user",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "How to fix this error?"}},
							},
						},
					},
					{
						Author: "assistant",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "This is a detailed solution that is longer than 20 characters to meet the requirement."}},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder:  &mockEmbedder{embedValue: defaultVector},
			store:     &mockStore{},
			wantSaved: true,
			wantError: false,
			checkSavedData: func(t *testing.T, saved []savedExperience) {
				if len(saved) != 1 {
					t.Errorf("Expected 1 saved experience, got %d", len(saved))
					return
				}
				if saved[0].pattern != "How to fix this error?" {
					t.Errorf("Expected pattern 'How to fix this error?', got %q", saved[0].pattern)
				}
				if saved[0].cause != "" {
					t.Errorf("Expected empty cause, got %q", saved[0].cause)
				}
				if saved[0].solution != "This is a detailed solution that is longer than 20 characters to meet the requirement." {
					t.Errorf("Unexpected solution: %q", saved[0].solution)
				}
			},
		},
		{
			name: "skip save when explicit save_experience tool was called",
			session: &mockSession{
				id:      "test-session-2",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "user",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "User question"}},
							},
						},
					},
					{
						Author: "assistant",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{
									{
										FunctionCall: &genai.FunctionCall{
											Name: "save_experience",
										},
									},
								},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder:  &mockEmbedder{embedValue: defaultVector},
			store:     &mockStore{},
			wantSaved: false,
			wantError: false,
		},
		{
			name: "skip save when agent response is too short",
			session: &mockSession{
				id:      "test-session-3",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "user",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "User question"}},
							},
						},
					},
					{
						Author: "assistant",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "Short"}},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder:  &mockEmbedder{embedValue: defaultVector},
			store:     &mockStore{},
			wantSaved: false,
			wantError: false,
		},
		{
			name: "skip save when no user query",
			session: &mockSession{
				id:      "test-session-4",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "assistant",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "This is a detailed response without a user query."}},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder:  &mockEmbedder{embedValue: defaultVector},
			store:     &mockStore{},
			wantSaved: false,
			wantError: false,
		},
		{
			name: "skip save when no agent response",
			session: &mockSession{
				id:      "test-session-5",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "user",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "User question"}},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder:  &mockEmbedder{embedValue: defaultVector},
			store:     &mockStore{},
			wantSaved: false,
			wantError: false,
		},
		{
			name: "error when embedding generation fails",
			session: &mockSession{
				id:      "test-session-6",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "user",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "User question"}},
							},
						},
					},
					{
						Author: "assistant",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "This is a detailed solution that is longer than 20 characters."}},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder: &mockEmbedder{
				embedError: errors.New("embedding failed"),
			},
			store:        &mockStore{},
			wantSaved:    false,
			wantError:    true,
			wantErrorMsg: "failed to generate embedding for session",
		},
		{
			name: "error when save fails",
			session: &mockSession{
				id:      "test-session-7",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "user",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "User question"}},
							},
						},
					},
					{
						Author: "assistant",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{{Text: "This is a detailed solution that is longer than 20 characters."}},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder: &mockEmbedder{embedValue: defaultVector},
			store: &mockStore{
				saveError: errors.New("database error"),
			},
			wantSaved:    false,
			wantError:    true,
			wantErrorMsg: "failed to save session to memory",
		},
		{
			name: "multiple text parts in content",
			session: &mockSession{
				id:      "test-session-8",
				appName: "test-app",
				userID:  "test-user",
				events: []*session.Event{
					{
						Author: "user",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{
									{Text: "First part "},
									{Text: "second part"},
								},
							},
						},
					},
					{
						Author: "assistant",
						LLMResponse: model.LLMResponse{
							Content: &genai.Content{
								Parts: []*genai.Part{
									{Text: "Response part 1 "},
									{Text: "and part 2 with enough length"},
								},
							},
						},
					},
				},
				lastTime: time.Now(),
			},
			embedder:  &mockEmbedder{embedValue: defaultVector},
			store:     &mockStore{},
			wantSaved: true,
			wantError: false,
			checkSavedData: func(t *testing.T, saved []savedExperience) {
				if len(saved) != 1 {
					t.Errorf("Expected 1 saved experience, got %d", len(saved))
					return
				}
				// Note: strings.Join adds a space between parts, so "First part " + "second part" = "First part  second part"
				expectedPattern := "First part  second part"
				if saved[0].pattern != expectedPattern {
					t.Errorf("Expected pattern %q, got %q", expectedPattern, saved[0].pattern)
				}
				expectedSolution := "Response part 1  and part 2 with enough length"
				if saved[0].solution != expectedSolution {
					t.Errorf("Expected solution %q, got %q", expectedSolution, saved[0].solution)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(tt.store, tt.embedder)

			err := service.AddSession(ctx, tt.session)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.wantErrorMsg != "" && !contains(err.Error(), tt.wantErrorMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.wantErrorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
			}

			if tt.wantSaved {
				if len(tt.store.savedExperiences) == 0 {
					t.Error("Expected experience to be saved, but none were saved")
					return
				}
				if tt.checkSavedData != nil {
					tt.checkSavedData(t, tt.store.savedExperiences)
				}
			} else {
				if len(tt.store.savedExperiences) > 0 {
					t.Errorf("Expected no experience to be saved, but %d were saved", len(tt.store.savedExperiences))
				}
			}
		})
	}
}

func TestService_Search(t *testing.T) {
	ctx := context.Background()
	defaultVector := []float32{0.1, 0.2, 0.3}

	tests := []struct {
		name         string
		request      *adkmemory.SearchRequest
		embedder     *mockEmbedder
		store        *mockStore
		wantError    bool
		wantErrorMsg string
		checkResult  func(*testing.T, *adkmemory.SearchResponse)
	}{
		{
			name: "successful search with results",
			request: &adkmemory.SearchRequest{
				Query: "test query",
			},
			embedder: &mockEmbedder{embedValue: defaultVector},
			store: &mockStore{
				searchResults: []Experience{
					{
						ID:              1,
						ErrorPattern:    "test error",
						RootCause:       "test cause",
						Solution:        "test solution",
						SimilarityScore: 0.95,
						OccurredAt:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					},
					{
						ID:              2,
						ErrorPattern:    "another error",
						RootCause:       "another cause",
						Solution:        "another solution",
						SimilarityScore: 0.85,
						OccurredAt:      time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			wantError: false,
			checkResult: func(t *testing.T, resp *adkmemory.SearchResponse) {
				if len(resp.Memories) != 2 {
					t.Errorf("Expected 2 memories, got %d", len(resp.Memories))
					return
				}

				// Check first memory
				mem1 := resp.Memories[0]
				if mem1.Author != "system" {
					t.Errorf("Expected author 'system', got %q", mem1.Author)
				}
				if mem1.Timestamp != time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) {
					t.Errorf("Unexpected timestamp for first memory")
				}
				if len(mem1.Content.Parts) == 0 {
					t.Error("Expected content parts, got none")
					return
				}
				text1 := mem1.Content.Parts[0].Text
				if !contains(text1, "问题: test error") {
					t.Errorf("Expected content to contain '问题: test error', got %q", text1)
				}
				if !contains(text1, "原因: test cause") {
					t.Errorf("Expected content to contain '原因: test cause', got %q", text1)
				}
				if !contains(text1, "解决方案: test solution") {
					t.Errorf("Expected content to contain '解决方案: test solution', got %q", text1)
				}
			},
		},
		{
			name: "successful search with no results",
			request: &adkmemory.SearchRequest{
				Query: "test query",
			},
			embedder: &mockEmbedder{embedValue: defaultVector},
			store: &mockStore{
				searchResults: []Experience{},
			},
			wantError: false,
			checkResult: func(t *testing.T, resp *adkmemory.SearchResponse) {
				if len(resp.Memories) != 0 {
					t.Errorf("Expected 0 memories, got %d", len(resp.Memories))
				}
			},
		},
		{
			name: "search with experience missing some fields",
			request: &adkmemory.SearchRequest{
				Query: "test query",
			},
			embedder: &mockEmbedder{embedValue: defaultVector},
			store: &mockStore{
				searchResults: []Experience{
					{
						ID:           1,
						ErrorPattern: "test error",
						// No RootCause
						Solution:   "test solution",
						OccurredAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					},
					{
						ID:        2,
						RootCause: "only cause",
						// No ErrorPattern or Solution
						OccurredAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			wantError: false,
			checkResult: func(t *testing.T, resp *adkmemory.SearchResponse) {
				if len(resp.Memories) != 2 {
					t.Errorf("Expected 2 memories, got %d", len(resp.Memories))
					return
				}

				// First memory should have pattern and solution
				text1 := resp.Memories[0].Content.Parts[0].Text
				if !contains(text1, "问题: test error") {
					t.Errorf("Expected content to contain '问题: test error', got %q", text1)
				}
				if !contains(text1, "解决方案: test solution") {
					t.Errorf("Expected content to contain '解决方案: test solution', got %q", text1)
				}

				// Second memory should have only cause
				text2 := resp.Memories[1].Content.Parts[0].Text
				if !contains(text2, "原因: only cause") {
					t.Errorf("Expected content to contain '原因: only cause', got %q", text2)
				}
			},
		},
		{
			name: "error when embedding generation fails",
			request: &adkmemory.SearchRequest{
				Query: "test query",
			},
			embedder: &mockEmbedder{
				embedError: errors.New("embedding failed"),
			},
			store:        &mockStore{},
			wantError:    true,
			wantErrorMsg: "failed to generate query embedding",
		},
		{
			name: "error when search fails",
			request: &adkmemory.SearchRequest{
				Query: "test query",
			},
			embedder: &mockEmbedder{embedValue: defaultVector},
			store: &mockStore{
				searchError: errors.New("database error"),
			},
			wantError:    true,
			wantErrorMsg: "failed to search similar issues",
		},
		{
			name: "skip experiences with empty content",
			request: &adkmemory.SearchRequest{
				Query: "test query",
			},
			embedder: &mockEmbedder{embedValue: defaultVector},
			store: &mockStore{
				searchResults: []Experience{
					{
						ID:         1,
						OccurredAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						// All fields empty
					},
					{
						ID:           2,
						ErrorPattern: "valid pattern",
						OccurredAt:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					},
				},
			},
			wantError: false,
			checkResult: func(t *testing.T, resp *adkmemory.SearchResponse) {
				// Should skip the first empty experience
				if len(resp.Memories) != 1 {
					t.Errorf("Expected 1 memory (empty one skipped), got %d", len(resp.Memories))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(tt.store, tt.embedder)

			resp, err := service.Search(ctx, tt.request)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.wantErrorMsg != "" && !contains(err.Error(), tt.wantErrorMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.wantErrorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if resp == nil {
					t.Error("Expected response, got nil")
					return
				}
				if tt.checkResult != nil {
					tt.checkResult(t, resp)
				}
			}
		})
	}
}

func TestNewService(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		store        Store
		cfg          config.Config
		wantError    bool
		wantErrorMsg string
	}{
		{
			name:  "successful creation",
			store: &mockStore{},
			cfg: config.Config{
				APIKey: "test-api-key",
			},
			wantError: false,
		},
		{
			name:  "error when GenAI client creation fails",
			store: &mockStore{},
			cfg: config.Config{
				APIKey: "", // Empty API key might cause error
			},
			wantError:    true,
			wantErrorMsg: "failed to create GenAI client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewService(ctx, tt.store, tt.cfg)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}
				if tt.wantErrorMsg != "" && !contains(err.Error(), tt.wantErrorMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.wantErrorMsg, err.Error())
				}
				if service != nil {
					t.Error("Expected service to be nil on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if service == nil {
					t.Error("Expected service, got nil")
					return
				}
				if service.store != tt.store {
					t.Error("Service store not set correctly")
				}
				if service.embedder == nil {
					t.Error("Service embedder not initialized")
				}
			}
		})
	}
}

// contains checks if a string contains a substring (case-sensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsHelper(s, substr))))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
