package alert

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go_base_skeleton/internal/config"
	"go_base_skeleton/internal/pkg/httpclient"
)

type recHTTPClient struct {
	mu             sync.Mutex
	postURL        string
	postBody       any
	postResult     any
	postErr        error
	postFormURL    string
	postFormData   map[string]string
	postFormResult any
	postFormErr    error
}

func (c *recHTTPClient) Get(_ context.Context, _ string, _ map[string]string, _ any, _ ...httpclient.Option) error {
	return errors.New("not implemented")
}

func (c *recHTTPClient) Post(_ context.Context, url string, body any, result any, opts ...httpclient.Option) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.postURL = url
	c.postBody = body
	c.postResult = result
	if out, ok := result.(*tgResponse); ok {
		out.OK = true
	}
	return c.postErr
}

func (c *recHTTPClient) PostForm(_ context.Context, url string, formData map[string]string, result any, _ ...httpclient.Option) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.postFormURL = url
	c.postFormData = map[string]string{}
	for k, v := range formData {
		c.postFormData[k] = v
	}
	c.postFormResult = result
	if out, ok := result.(*tgResponse); ok {
		out.OK = true
	}
	return c.postFormErr
}

func (c *recHTTPClient) snapshotPost() (string, any, any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.postURL, c.postBody, c.postResult
}

func (c *recHTTPClient) snapshotPostForm() (string, map[string]string, any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{}
	for k, v := range c.postFormData {
		out[k] = v
	}
	return c.postFormURL, out, c.postFormResult
}

type recSink struct {
	mu   sync.Mutex
	got  []Alert
	err  error
	name string // 若空，Name 返回 "test"
}

func (r *recSink) Name() string {
	if r.name != "" {
		return r.name
	}
	return "test"
}

func (r *recSink) Send(_ context.Context, a Alert) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, a)
	return r.err
}

func (r *recSink) all() []Alert {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Alert(nil), r.got...)
}

func waitAsync(t *testing.T) {
	t.Helper()
	time.Sleep(100 * time.Millisecond)
}

func TestDeduplicator_Try_Commit(t *testing.T) {
	d := NewDeduplicator(true, 50*time.Millisecond)
	k := "same"
	assert.True(t, d.Try(k))
	assert.True(t, d.Try(k), "before Commit, second Try is still true")
	d.Commit(k)
	assert.False(t, d.Try(k))
	assert.True(t, d.Try("other"))
	time.Sleep(60 * time.Millisecond)
	assert.True(t, d.Try(k))
}

func TestBuildDedupKey_Stable(t *testing.T) {
	a := Alert{Level: LevelError, Title: "t", Message: "m", Extra: map[string]any{"dedup_key": "x"}}
	b := BuildDedupKey(a)
	c := BuildDedupKey(a)
	assert.Equal(t, b, c)
}

func TestNotifier_Fanout_TwoSinks_Both(t *testing.T) {
	s1, s2 := &recSink{name: "a"}, &recSink{name: "b"}
	fo := &Fanout{
		Sinks:          []Sink{s1, s2},
		PerSinkTimeout: 2 * time.Second,
	}
	a := Alert{Level: LevelInfo, Title: "hi", Message: "body"}
	fo.SendAll(context.Background(), a)
	assert.Len(t, s1.all(), 1)
	assert.Len(t, s2.all(), 1)
	assert.Equal(t, "hi", s1.all()[0].Title)
}

func TestNotifier_Fanout_OneFails_Other(t *testing.T) {
	s1, s2 := &recSink{name: "ok"}, &recSink{name: "bad", err: errors.New("sink err")}
	fo := &Fanout{
		Sinks:          []Sink{s1, s2},
		PerSinkTimeout: 2 * time.Second,
	}
	fo.SendAll(context.Background(), Alert{Title: "t"})
	g1, g2 := s1.all(), s2.all()
	assert.Len(t, g1, 1, "succeeding sink should still be called")
	assert.Len(t, g2, 1, "failing sink should be invoked as well")
}

// TestNotifier_Send_WebhookReceiveBody 验证 Notifier 会通过统一 HTTPClient 投递 webhook。
func TestNotifier_Send_WebhookReceiveBody(t *testing.T) {
	httpCli := &recHTTPClient{}
	ac := &config.AlertConfig{
		Enabled:      true,
		DedupEnabled: false,
		Webhook:      config.AlertWebhookConfig{Enabled: true, URL: "https://example.com/webhook"},
	}
	n := New(zap.NewNop(), ac, httpCli)
	n.Send(context.Background(), Alert{Level: LevelError, Title: "t1", Message: "m1"})
	waitAsync(t)

	url, body, result := httpCli.snapshotPost()
	assert.Equal(t, "https://example.com/webhook", url)
	assert.IsType(t, webhookPayload{}, body)
	assert.IsType(t, &map[string]any{}, result)
}

// TestNotifier_Send_Integration_RecSinks 使用自定义 Notifier 装配
func TestNotifier_Integration_RecSinks(t *testing.T) {
	s1, s2 := &recSink{name: "a"}, &recSink{name: "b"}
	d := NewDeduplicator(false, 0)
	fo := &Fanout{
		Sinks:          []Sink{s1, s2},
		PerSinkTimeout: time.Second,
	}
	ac := &config.AlertConfig{Enabled: true, DedupEnabled: false}
	n := &Notifier{log: zap.NewNop(), dedup: d, fo: fo, ac: ac}
	n.Send(context.Background(), Alert{Title: "x"})
	waitAsync(t)
	assert.Len(t, s1.all(), 1)
	assert.Len(t, s2.all(), 1)
}

func TestNotifier_Integration_Throttled(t *testing.T) {
	s1 := &recSink{name: "a"}
	fo := &Fanout{
		Sinks:          []Sink{s1},
		PerSinkTimeout: time.Second,
	}
	d := NewDeduplicator(true, 10*time.Minute)
	ac := &config.AlertConfig{Enabled: true, DedupEnabled: true, DedupWindow: 10 * time.Minute}
	n := &Notifier{log: zap.NewNop(), dedup: d, fo: fo, ac: ac}
	a := Alert{Level: LevelError, Title: "T", Message: "M"}
	n.Send(context.Background(), a)
	waitAsync(t)                    // 首路投递成功并 Commit
	n.Send(context.Background(), a) // 同 dedup，应被 Try 挡掉
	waitAsync(t)
	assert.Len(t, s1.all(), 1, "second Send should be throttled after first Commit")
}

// TestNotifier_Dedup_AllSinkFailNoCommit_ThirdTryGoes 全失败不 Commit，冷却期内仍可再次尝试
func TestNotifier_Dedup_AllSinkFailNoCommit_ThirdTryGoes(t *testing.T) {
	s1 := &recSink{name: "a", err: errors.New("fail")}
	fo := &Fanout{Sinks: []Sink{s1}, PerSinkTimeout: time.Second}
	d := NewDeduplicator(true, time.Hour) // 成功一次则长时间冷却；此处不应 Commit
	ac := &config.AlertConfig{Enabled: true, DedupEnabled: true, DedupWindow: time.Hour}
	n := &Notifier{log: zap.NewNop(), dedup: d, fo: fo, ac: ac}
	aa := Alert{Level: LevelError, Title: "E", Message: "m"}
	n.Send(context.Background(), aa)
	waitAsync(t)
	assert.Len(t, s1.all(), 1)
	n.Send(context.Background(), aa)
	waitAsync(t)
	assert.Len(t, s1.all(), 2, "all sinks failed, no Commit, should allow second try")
}

func TestNotifier_NoOp_Disabled(t *testing.T) {
	s := &recSink{name: "x"}
	fo := &Fanout{Sinks: []Sink{s}, PerSinkTimeout: time.Second}
	n := &Notifier{log: zap.NewNop(), fo: fo, ac: &config.AlertConfig{Enabled: false}}
	n.Send(context.Background(), Alert{Title: "x"})
	waitAsync(t)
	assert.Empty(t, s.all())
}

func TestWebhookSink_JSON(t *testing.T) {
	httpCli := &recHTTPClient{}
	ws := &WebhookSink{Client: httpCli, URL: "https://example.com/webhook"}
	err := ws.Send(context.Background(), Alert{Level: LevelInfo, Title: "b", Message: "c", TraceID: "tid"})
	require.NoError(t, err)
	url, body, result := httpCli.snapshotPost()
	require.Equal(t, "https://example.com/webhook", url)
	require.IsType(t, webhookPayload{}, body)
	payload := body.(webhookPayload)
	assert.Equal(t, "tid", payload.TraceID)
	assert.Equal(t, "b", payload.Title)
	assert.IsType(t, &map[string]any{}, result)
}

func TestTelegramSink_UsesFormData(t *testing.T) {
	httpCli := &recHTTPClient{}
	sink := &TelegramSink{
		BotToken: "token",
		ChatID:   "123",
		Client:   httpCli,
	}

	err := sink.Send(context.Background(), Alert{Title: "hello", Message: "world"})

	require.NoError(t, err)
	url, formData, result := httpCli.snapshotPostForm()
	assert.Equal(t, "https://api.telegram.org/bottoken/sendMessage", url)
	assert.Equal(t, map[string]string{"chat_id": "123", "text": "hello\n\nworld"}, formData)
	assert.IsType(t, &tgResponse{}, result)
}

func TestSink_Name_UsesConstants(t *testing.T) {
	assert.Equal(t, SinkWebhookName, (&WebhookSink{}).Name())
	assert.Equal(t, SinkTelegramName, (&TelegramSink{}).Name())
}

func TestBuildDedupKey_WithoutTracesMerges(t *testing.T) {
	// 相同业务键不同 trace：dedup key 同（不含 trace）
	k1 := BuildDedupKey(Alert{Level: LevelError, Title: "T", Message: "M", TraceID: "a"})
	k2 := BuildDedupKey(Alert{Level: LevelError, Title: "T", Message: "M", TraceID: "b"})
	assert.Equal(t, k1, k2)
}
