package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wzfukui/agent-native-im/internal/auth"
	"github.com/wzfukui/agent-native-im/internal/model"
)

// POST /conversations/:id/tasks
func (s *Server) HandleCreateTask(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, _ := s.Store.IsParticipant(ctx, convID, entityID)
	if !ok {
		Fail(c, http.StatusForbidden, "not a participant")
		return
	}

	var req struct {
		Title        string  `json:"title" binding:"required"`
		Description  string  `json:"description"`
		AssigneeID   *int64  `json:"assignee_id"`
		Priority     string  `json:"priority"`
		DueDate      *string `json:"due_date"`
		ParentTaskID *int64  `json:"parent_task_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "title is required")
		return
	}

	priority := model.PriorityMedium
	switch model.TaskPriority(req.Priority) {
	case model.PriorityLow, model.PriorityHigh:
		priority = model.TaskPriority(req.Priority)
	}

	task := &model.Task{
		ConversationID: convID,
		Title:          req.Title,
		Description:    req.Description,
		AssigneeID:     req.AssigneeID,
		Status:         model.TaskPending,
		Priority:       priority,
		ParentTaskID:   req.ParentTaskID,
		CreatedBy:      entityID,
	}

	if req.DueDate != nil {
		t, err := time.Parse(time.RFC3339, *req.DueDate)
		if err == nil {
			task.DueDate = &t
		}
	}

	if err := s.Store.CreateTask(ctx, task); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	// Enrich with creator
	creator, _ := s.Store.GetEntityByID(ctx, entityID)
	task.Creator = creator
	if req.AssigneeID != nil {
		assignee, _ := s.Store.GetEntityByID(ctx, *req.AssigneeID)
		task.Assignee = assignee
	}

	// Broadcast
	if s.Hub != nil {
		s.Hub.BroadcastEvent(convID, "task.updated", map[string]interface{}{
			"action": "created",
			"task":   task,
		})
	}

	creatorName := getEntityDisplayName(creator)
	s.broadcastSystemMessage(c, convID, entityID,
		fmt.Sprintf("%s created task: %s", creatorName, task.Title))

	OK(c, http.StatusCreated, task)
}

// GET /conversations/:id/tasks?status=
func (s *Server) HandleListTasks(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid conversation id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	ok, _ := s.Store.IsParticipant(ctx, convID, entityID)
	if !ok {
		Fail(c, http.StatusForbidden, "not a participant")
		return
	}

	status := c.Query("status")
	tasks, err := s.Store.ListTasksByConversation(ctx, convID, status)
	if err != nil {
		Fail(c, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	if tasks == nil {
		tasks = []*model.Task{}
	}

	// Enrich with assignees and creators
	for _, t := range tasks {
		if t.AssigneeID != nil {
			t.Assignee, _ = s.Store.GetEntityByID(ctx, *t.AssigneeID)
		}
		t.Creator, _ = s.Store.GetEntityByID(ctx, t.CreatedBy)
	}

	OK(c, http.StatusOK, tasks)
}

// GET /tasks/:id
func (s *Server) HandleGetTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid task id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	task, err := s.Store.GetTask(ctx, taskID)
	if err != nil {
		Fail(c, http.StatusNotFound, "task not found")
		return
	}

	ok, _ := s.Store.IsParticipant(ctx, task.ConversationID, entityID)
	if !ok {
		Fail(c, http.StatusForbidden, "not a participant")
		return
	}

	// Enrich
	if task.AssigneeID != nil {
		task.Assignee, _ = s.Store.GetEntityByID(ctx, *task.AssigneeID)
	}
	task.Creator, _ = s.Store.GetEntityByID(ctx, task.CreatedBy)

	OK(c, http.StatusOK, task)
}

// PUT /tasks/:id
func (s *Server) HandleUpdateTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid task id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	task, err := s.Store.GetTask(ctx, taskID)
	if err != nil {
		Fail(c, http.StatusNotFound, "task not found")
		return
	}

	// Permission: creator, assignee, or owner/admin
	isCreator := task.CreatedBy == entityID
	isAssignee := task.AssigneeID != nil && *task.AssigneeID == entityID
	if !isCreator && !isAssignee {
		p, err := s.Store.GetParticipant(ctx, task.ConversationID, entityID)
		if err != nil || p == nil || (p.Role != model.RoleOwner && p.Role != model.RoleAdmin) {
			Fail(c, http.StatusForbidden, "only creator, assignee, or admin can update this task")
			return
		}
	}

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		AssigneeID  *int64  `json:"assignee_id"`
		Status      *string `json:"status"`
		Priority    *string `json:"priority"`
		DueDate     *string `json:"due_date"`
		SortOrder   *int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.AssigneeID != nil {
		task.AssigneeID = req.AssigneeID
	}
	if req.Status != nil {
		newStatus := model.TaskStatus(*req.Status)
		switch newStatus {
		case model.TaskPending, model.TaskInProgress, model.TaskDone, model.TaskCancelled:
			task.Status = newStatus
			if newStatus == model.TaskDone || newStatus == model.TaskCancelled {
				now := time.Now()
				task.CompletedAt = &now
			} else {
				task.CompletedAt = nil
			}
		}
	}
	if req.Priority != nil {
		p := model.TaskPriority(*req.Priority)
		switch p {
		case model.PriorityLow, model.PriorityMedium, model.PriorityHigh:
			task.Priority = p
		}
	}
	if req.DueDate != nil {
		t, err := time.Parse(time.RFC3339, *req.DueDate)
		if err == nil {
			task.DueDate = &t
		}
	}
	if req.SortOrder != nil {
		task.SortOrder = *req.SortOrder
	}

	if err := s.Store.UpdateTask(ctx, task); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to update task")
		return
	}

	// Enrich
	if task.AssigneeID != nil {
		task.Assignee, _ = s.Store.GetEntityByID(ctx, *task.AssigneeID)
	}
	task.Creator, _ = s.Store.GetEntityByID(ctx, task.CreatedBy)

	// Broadcast
	if s.Hub != nil {
		s.Hub.BroadcastEvent(task.ConversationID, "task.updated", map[string]interface{}{
			"action": "updated",
			"task":   task,
		})
	}

	OK(c, http.StatusOK, task)
}

// DELETE /tasks/:id
func (s *Server) HandleDeleteTask(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		Fail(c, http.StatusBadRequest, "invalid task id")
		return
	}

	entityID := auth.GetEntityID(c)
	ctx := c.Request.Context()

	task, err := s.Store.GetTask(ctx, taskID)
	if err != nil {
		Fail(c, http.StatusNotFound, "task not found")
		return
	}

	// Permission: creator or owner/admin
	if task.CreatedBy != entityID {
		p, err := s.Store.GetParticipant(ctx, task.ConversationID, entityID)
		if err != nil || p == nil || (p.Role != model.RoleOwner && p.Role != model.RoleAdmin) {
			Fail(c, http.StatusForbidden, "only creator or admin can delete this task")
			return
		}
	}

	if err := s.Store.DeleteTask(ctx, taskID); err != nil {
		Fail(c, http.StatusInternalServerError, "failed to delete task")
		return
	}

	// Broadcast
	if s.Hub != nil {
		s.Hub.BroadcastEvent(task.ConversationID, "task.updated", map[string]interface{}{
			"action":  "deleted",
			"task_id": taskID,
		})
	}

	OK(c, http.StatusOK, nil)
}
