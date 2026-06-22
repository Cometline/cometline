package discord

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/cometline/cometmind/internal/gateway"
)

func workspaceOptionLabel(path string) string {
	base := filepath.Base(strings.TrimRight(path, "/"))
	if base == "" || base == "." {
		base = path
	}
	if len(base) > 100 {
		return "…" + base[len(base)-99:]
	}
	return base
}

func buildJobProposalComponents(proposal *gateway.PendingJobProposal, workspacePaths []string) []discordgo.MessageComponent {
	rows := make([]discordgo.MessageComponent, 0, 3)
	if len(workspacePaths) > 0 {
		options := make([]discordgo.SelectMenuOption, 0, len(workspacePaths))
		for i, path := range workspacePaths {
			if i >= 25 {
				break
			}
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			options = append(options, discordgo.SelectMenuOption{
				Label:       workspaceOptionLabel(path),
				Value:       path,
				Description: truncateDescription(path, 100),
				Default:     path == proposal.SelectedWorkspace,
			})
		}
		if len(options) > 0 {
			placeholder := "Select workspace"
			if proposal.DefaultWorkspace != "" {
				placeholder = "Default: " + workspaceOptionLabel(proposal.DefaultWorkspace)
			}
			rows = append(rows, discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    gateway.JobProposalSelectCustomID(proposal.ID),
						Placeholder: placeholder,
						Options:     options,
					},
				},
			})
		}
	}
	rows = append(rows, discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.Button{
				Label:    "Confirm",
				Style:    discordgo.PrimaryButton,
				CustomID: gateway.JobProposalConfirmCustomID(proposal.ID),
			},
			discordgo.Button{
				Label:    "Cancel",
				Style:    discordgo.SecondaryButton,
				CustomID: gateway.JobProposalCancelCustomID(proposal.ID),
			},
		},
	})
	return rows
}

func truncateDescription(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return "…" + text[len(text)-max+1:]
}

func formatJobProposalContent(proposal *gateway.PendingJobProposal) string {
	var b strings.Builder
	b.WriteString("**Confirm job**\n\n")
	b.WriteString(proposal.Description)
	if proposal.DefinitionOfDone != "" {
		b.WriteString("\n\n**Definition of done**\n")
		b.WriteString(proposal.DefinitionOfDone)
	}
	if proposal.DefaultWorkspace != "" {
		fmt.Fprintf(&b, "\n\nDefault workspace: `%s`", proposal.DefaultWorkspace)
	}
	b.WriteString("\n\nPick a workspace, then click **Confirm**.")
	return b.String()
}

// DeliverJobProposal posts a Discord message with workspace selection components.
func (a *Adapter) DeliverJobProposal(ctx context.Context, msg gateway.OutboundMessage, proposal *gateway.PendingJobProposal, workspacePaths []string) error {
	_ = ctx
	if a.Session == nil || proposal == nil {
		return fmt.Errorf("discord session or proposal missing")
	}
	dest := deliveryChannelID(msg)
	_, err := a.Session.ChannelMessageSendComplex(dest, &discordgo.MessageSend{
		Content:    formatJobProposalContent(proposal),
		Components: buildJobProposalComponents(proposal, workspacePaths),
	})
	return err
}

func (a *Adapter) handleJobProposalComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	action, proposalID, ok := gateway.ParseJobProposalCustomID(data.CustomID)
	if !ok {
		return
	}
	msg := routingInboundMessage(s, i)

	switch action {
	case "select":
		if len(data.Values) == 0 {
			respondEphemeral(s, i, "No workspace selected.")
			return
		}
		if a.onJobProposalSelect == nil {
			respondEphemeral(s, i, "Job proposals are not configured.")
			return
		}
		text, err := a.onJobProposalSelect(proposalID, data.Values[0])
		if err != nil {
			respondEphemeral(s, i, err.Error())
			return
		}
		respondEphemeral(s, i, text)
	case "confirm":
		if a.onJobProposalConfirm == nil {
			respondEphemeral(s, i, "Job proposals are not configured.")
			return
		}
		text, err := a.onJobProposalConfirm(context.Background(), msg, proposalID)
		if err != nil {
			respondEphemeral(s, i, err.Error())
			return
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    text + "\n\n_(proposal closed)_",
				Components: []discordgo.MessageComponent{},
			},
		})
	case "cancel":
		if a.onJobProposalCancel != nil {
			_ = a.onJobProposalCancel(proposalID)
		}
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Job proposal dismissed.",
				Components: []discordgo.MessageComponent{},
			},
		})
	}
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
