package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/gateway"
)

func TestHandleStopCommandDefersBeforeWaitingAndEditsResponse(t *testing.T) {
	deferred := make(chan map[string]any, 1)
	edited := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/callback":
			deferred <- body
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && r.URL.Path == "/original":
			edited <- body
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"response"}`))
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	oldResponse := discordgo.EndpointInteractionResponse
	oldWebhookMessage := discordgo.EndpointWebhookMessage
	discordgo.EndpointInteractionResponse = func(_, _ string) string { return server.URL + "/callback" }
	discordgo.EndpointWebhookMessage = func(_, _, _ string) string { return server.URL + "/original" }
	t.Cleanup(func() {
		discordgo.EndpointInteractionResponse = oldResponse
		discordgo.EndpointWebhookMessage = oldWebhookMessage
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New() error = %v", err)
	}
	releaseStop := make(chan struct{})
	adapter := &Adapter{Session: session}
	adapter.SetStopHandler(func(context.Context, gateway.InboundMessage) (string, error) {
		<-releaseStop
		return "Stopped the active turn.", nil
	})
	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:        "interaction-1",
		AppID:     "app-1",
		Token:     "token-1",
		ChannelID: "channel-1",
		User:      &discordgo.User{ID: "user-1"},
	}}
	handlerDone := make(chan struct{})
	go func() {
		adapter.handleStopCommand(session, interaction, discordgo.ApplicationCommandInteractionData{})
		close(handlerDone)
	}()

	deferBody := <-deferred
	if got := deferBody["type"]; got != float64(discordgo.InteractionResponseDeferredChannelMessageWithSource) {
		t.Fatalf("deferred response type = %#v", got)
	}
	select {
	case <-edited:
		t.Fatal("interaction response was edited before stop cleanup completed")
	default:
	}

	close(releaseStop)
	editBody := <-edited
	if got := editBody["content"]; got != "Stopped the active turn." {
		t.Fatalf("edited content = %#v", got)
	}
	<-handlerDone
}
