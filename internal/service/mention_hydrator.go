package service

import (
	"context"

	"ipmanlk/plume/internal/domain"
	"ipmanlk/plume/internal/port"
)

// mentionHydrator resolves <@type:id> tokens in text into a domain.Mentions
// payload (users/channels/projects/tasks → labels). Shared by the comment,
// task, and message services so all render mention chips identically on the
// frontend without re-fetching labels.
type mentionHydrator struct {
	userRepo    port.UserRepository
	projectRepo port.ProjectRepository
	taskRepo    port.TaskRepository
	convRepo    port.ConversationRepository
}

func newMentionHydrator(
	userRepo port.UserRepository,
	projectRepo port.ProjectRepository,
	taskRepo port.TaskRepository,
	convRepo port.ConversationRepository,
) mentionHydrator {
	return mentionHydrator{
		userRepo:    userRepo,
		projectRepo: projectRepo,
		taskRepo:    taskRepo,
		convRepo:    convRepo,
	}
}

// hydrateMany resolves tokens across multiple content strings in bulk (single
// batch of repo lookups), returning one *domain.Mentions per input. This
// mirrors the per-message hydration pattern used by MessageService and
// CommentService.
func (h mentionHydrator) hydrateMany(ctx context.Context, orgID string, contents []string) []*domain.Mentions {
	results := make([]*domain.Mentions, len(contents))
	if len(contents) == 0 {
		return results
	}

	allUserIDs := make(map[string]bool)
	allProjectIDs := make(map[string]bool)
	allTaskIDs := make(map[string]bool)
	allChannelIDs := make(map[string]bool)

	for _, content := range contents {
		for _, t := range domain.ParseMentionTokens(content) {
			switch t.Type {
			case "user":
				allUserIDs[t.ID] = true
			case "project":
				allProjectIDs[t.ID] = true
			case "task":
				allTaskIDs[t.ID] = true
			case "channel":
				allChannelIDs[t.ID] = true
			}
		}
	}

	userNames := make(map[string]string)
	if len(allUserIDs) > 0 {
		ids := keys(allUserIDs)
		if users, err := h.userRepo.ListByIDs(ctx, ids); err == nil {
			for _, u := range users {
				userNames[u.ID] = u.Name
			}
		}
	}

	projectNames := make(map[string]string)
	if len(allProjectIDs) > 0 {
		ids := keys(allProjectIDs)
		if projects, err := h.projectRepo.ListByIDs(ctx, orgID, ids); err == nil {
			for _, p := range projects {
				projectNames[p.ID] = p.Name
			}
		}
	}

	taskMentions := make(map[string]domain.TaskMention)
	if len(allTaskIDs) > 0 {
		ids := keys(allTaskIDs)
		if tasks, err := h.taskRepo.ListByIDs(ctx, orgID, ids); err == nil {
			for _, t := range tasks {
				taskMentions[t.ID] = domain.TaskMention{Title: t.Title, ProjectID: t.ProjectID}
			}
		}
	}

	channelNames := make(map[string]string)
	if len(allChannelIDs) > 0 {
		ids := keys(allChannelIDs)
		if convs, err := h.convRepo.ListByIDs(ctx, orgID, ids); err == nil {
			for _, c := range convs {
				channelNames[c.ID] = c.Name
			}
		}
	}

	noTokens := len(allUserIDs) == 0 && len(allProjectIDs) == 0 &&
		len(allTaskIDs) == 0 && len(allChannelIDs) == 0

	for i, content := range contents {
		if noTokens {
			results[i] = &domain.Mentions{}
			continue
		}
		msgUsers := make(map[string]string)
		msgProjects := make(map[string]string)
		msgTasks := make(map[string]domain.TaskMention)
		msgChannels := make(map[string]string)

		for _, t := range domain.ParseMentionTokens(content) {
			switch t.Type {
			case "user":
				if name, ok := userNames[t.ID]; ok {
					msgUsers[t.ID] = name
				}
			case "project":
				if name, ok := projectNames[t.ID]; ok {
					msgProjects[t.ID] = name
				}
			case "task":
				if tm, ok := taskMentions[t.ID]; ok {
					msgTasks[t.ID] = tm
				}
			case "channel":
				if name, ok := channelNames[t.ID]; ok {
					msgChannels[t.ID] = name
				}
			}
		}

		results[i] = &domain.Mentions{
			Users:    msgUsers,
			Projects: msgProjects,
			Tasks:    msgTasks,
			Channels: msgChannels,
		}
	}

	return results
}

func keys[V any](m map[string]V) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}
