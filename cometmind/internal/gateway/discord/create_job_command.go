package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/gateway"
)

func createJobApplicationCommand() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "create-job",
		Description: "Create a global todo job with workspace selection",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "description",
				Description: "What needs to be done",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "definition_of_done",
				Description: "How to know the job is finished",
				Required:    false,
			},
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "workspace",
				Description:  "Workspace path (defaults to session workspace)",
				Required:     false,
				Autocomplete: true,
			},
		},
	}
}

func (a *Adapter) handleCreateJobCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	description := ""
	dod := ""
	workspace := ""
	for _, opt := range data.Options {
		if opt.Type != discordgo.ApplicationCommandOptionString {
			continue
		}
		switch opt.Name {
		case "description":
			description = strings.TrimSpace(opt.StringValue())
		case "definition_of_done":
			dod = strings.TrimSpace(opt.StringValue())
		case "workspace":
			workspace = strings.TrimSpace(opt.StringValue())
		}
	}
	if a.onCreateJob == nil {
		respondEphemeral(s, i, "Job creation is not configured.")
		return
	}
	msg := routingInboundMessage(s, i)
	text, err := a.onCreateJob(context.Background(), msg, description, dod, workspace)
	if err != nil {
		respondEphemeral(s, i, fmt.Sprintf("Failed to create job: %v", err))
		return
	}
	respondEphemeral(s, i, text)
}

func (a *Adapter) handleCreateJobAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if a.onSuggest == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}
	query := ""
	for _, opt := range data.Options {
		if opt.Name == "workspace" && opt.Focused {
			query = opt.StringValue()
			break
		}
	}
	paths, err := a.onSuggest(context.Background(), query)
	if err != nil {
		paths = nil
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(paths))
	for _, path := range paths {
		if len(choices) >= 25 {
			break
		}
		name := path
		if len(name) > 100 {
			name = "…" + name[len(name)-99:]
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: path,
		})
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// SetCreateJobHandler registers the callback used for /create-job slash commands.
func (a *Adapter) SetCreateJobHandler(fn func(context.Context, gateway.InboundMessage, string, string, string) (string, error)) {
	a.onCreateJob = fn
}

// SetJobProposalHandlers registers callbacks for job proposal component interactions.
func (a *Adapter) SetJobProposalHandlers(
	onSelect func(string, string) (string, error),
	onConfirm func(context.Context, gateway.InboundMessage, string) (string, error),
	onCancel func(string) error,
) {
	a.onJobProposalSelect = onSelect
	a.onJobProposalConfirm = onConfirm
	a.onJobProposalCancel = onCancel
}
