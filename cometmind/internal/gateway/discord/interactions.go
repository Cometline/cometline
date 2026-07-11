package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (a *Adapter) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommandAutocomplete:
		a.handleAutocomplete(s, i)
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()
		switch data.Name {
		case "thread":
			a.handleThreadCommand(s, i, data)
		case "create-skill":
			a.handleCreateSkillCommand(s, i, data)
		case "clear":
			a.handleClearCommand(s, i, data)
		case "stop":
			a.handleStopCommand(s, i, data)
		case "change":
			a.handleChangeCommand(s, i, data)
		case "jobs":
			a.handleJobsCommand(s, i, data)
		case "create-job":
			a.handleCreateJobCommand(s, i, data)
		}
	case discordgo.InteractionMessageComponent:
		a.handleJobProposalComponent(s, i)
	}
}

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
