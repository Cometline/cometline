package discord

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/gateway"
	"github.com/cometline/cometmind/internal/logging"
	skillpkg "github.com/cometline/cometmind/internal/skills"
)

func (a *Adapter) handleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	switch data.Name {
	case "change":
		a.handleChangeAutocomplete(s, i, data)
	case "jobs":
		a.handleJobsAutocomplete(s, i, data)
	case "create-job":
		a.handleCreateJobAutocomplete(s, i, data)
	default:
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
	}
}

func (a *Adapter) handleChangeAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if a.onSuggest == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	query := ""
	for _, opt := range data.Options {
		if opt.Name == "path" && opt.Focused {
			query = opt.StringValue()
			break
		}
	}

	paths, err := a.onSuggest(context.Background(), query)
	if err != nil {
		logging.L().Warn("discord.autocomplete.workspace_failed", "error", err)
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

func (a *Adapter) handleJobsAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	if a.jobSuggest == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}
	query := ""
	for _, opt := range data.Options {
		if opt.Name == "job" && opt.Focused {
			query = opt.StringValue()
			break
		}
	}
	items, err := a.jobSuggest(context.Background(), query)
	if err != nil {
		logging.L().Warn("discord.autocomplete.job_failed", "error", err)
		items = nil
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(items))
	for _, job := range items {
		if len(choices) >= 25 {
			break
		}
		name := job.Description
		if len(name) > 100 {
			name = name[:97] + "..."
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: job.ID,
		})
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func (a *Adapter) handleChangeCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	path := ""
	for _, opt := range data.Options {
		if opt.Name == "path" && opt.Type == discordgo.ApplicationCommandOptionString {
			path = strings.TrimSpace(opt.StringValue())
		}
	}
	if path == "" {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Workspace path is required.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if a.onChange == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Workspace switching is not configured.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	msg := routingInboundMessage(s, i)
	text, err := a.onChange(context.Background(), msg, path)
	if err != nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Failed to change workspace: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: text,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (a *Adapter) handleClearCommand(s *discordgo.Session, i *discordgo.InteractionCreate, _ discordgo.ApplicationCommandInteractionData) {
	if a.onClear == nil {
		respondEphemeral(s, i, "Session clearing is not configured.")
		return
	}

	msg := routingInboundMessage(s, i)
	text, err := a.onClear(context.Background(), msg)
	if err != nil {
		respondEphemeral(s, i, fmt.Sprintf("Failed to clear session: %v", err))
		return
	}

	respondEphemeral(s, i, text)
}

func (a *Adapter) handleStopCommand(s *discordgo.Session, i *discordgo.InteractionCreate, _ discordgo.ApplicationCommandInteractionData) {
	if a.onStop == nil {
		respondEphemeral(s, i, "Stopping turns is not configured.")
		return
	}

	msg := routingInboundMessage(s, i)
	text, err := a.onStop(context.Background(), msg)
	if err != nil {
		respondEphemeral(s, i, fmt.Sprintf("Failed to stop turn: %v", err))
		return
	}
	respondEphemeral(s, i, text)
}

func (a *Adapter) handleJobsCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	jobID := ""
	for _, opt := range data.Options {
		if opt.Name == "job" && opt.Type == discordgo.ApplicationCommandOptionString {
			jobID = strings.TrimSpace(opt.StringValue())
		}
	}
	if a.onJobs == nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Jobs are not configured.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	msg := routingInboundMessage(s, i)
	reply, runPrompt, err := a.onJobs(context.Background(), msg, jobID)
	if err != nil {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("Jobs command failed: %v", err),
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	flags := discordgo.MessageFlagsEphemeral
	if runPrompt != "" {
		flags = 0
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: reply,
			Flags:   flags,
		},
	})
	if runPrompt != "" && a.onInbound != nil {
		msg.Text = runPrompt
		a.onInbound(context.Background(), msg)
	}
}

func (a *Adapter) handleCreateSkillCommand(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	request := ""
	for _, opt := range data.Options {
		if opt.Name == "request" && opt.Type == discordgo.ApplicationCommandOptionString {
			if v := strings.TrimSpace(opt.StringValue()); v != "" {
				request = v
			}
		}
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Creating skill… CometMind will reply in this channel.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})

	if a.onInbound == nil {
		return
	}

	parentChannelID := ""
	if i.GuildID != "" {
		if ch, err := s.Channel(i.ChannelID); err == nil && ch != nil && ch.ParentID != "" {
			parentChannelID = ch.ParentID
		}
	}
	routingChannelID, threadID := discordRoutingIDs(i.ChannelID, parentChannelID)
	userID := interactionUserID(i)
	if userID == "" {
		return
	}

	a.onInbound(context.Background(), gateway.InboundMessage{
		Platform:        PlatformName,
		GuildID:         i.GuildID,
		ParentChannelID: parentChannelID,
		UserID:          userID,
		ChannelID:       routingChannelID,
		ThreadID:        threadID,
		Text:            skillpkg.ExpandCreateSkillCommand(request),
		Mentioned:       true,
	})
}
