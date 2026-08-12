package service

import (
	"sync"
	"time"

	"study-english/server/internal/model"
)

// TaskStatus 任务状态
const (
	TaskRunning = "running"
	TaskDone    = "done"
	TaskError   = "error"
)

// Task 后台任务（异步拉取课文等）
type Task struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"` // pull
	LessonID   uint                   `json:"lesson_id"`
	Status     string                 `json:"status"` // running / done / error
	Error      string                 `json:"error,omitempty"`
	Content    *model.AIContent       `json:"content,omitempty"` // 完成时的结果
	Imported   *model.ImportedContent `json:"imported,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	FinishedAt *time.Time             `json:"finished_at,omitempty"`
}

// TaskService 后台任务管理：goroutine 独立执行，不受前端连接影响（真正的异步）
type TaskService struct {
	mu      sync.Mutex
	tasks   map[string]*Task
	running map[uint]string // lessonID → taskID（同课时并发拉取去重）
	seq     uint64
}

func NewTaskService() *TaskService {
	return &TaskService{
		tasks:   map[string]*Task{},
		running: map[uint]string{},
	}
}

// StartPull 启动异步拉取任务：若该课时已在拉取，返回同一任务
func (s *TaskService) StartPull(lessonID uint, fn func() (*model.AIContent, error)) string {
	s.mu.Lock()
	if id, ok := s.running[lessonID]; ok {
		s.mu.Unlock()
		return id
	}
	s.seq++
	task := &Task{
		ID:        "task-" + itoa(s.seq),
		Kind:      "pull",
		LessonID:  lessonID,
		Status:    TaskRunning,
		CreatedAt: time.Now(),
	}
	s.tasks[task.ID] = task
	s.running[lessonID] = task.ID
	s.mu.Unlock()

	// 后台 goroutine：与 HTTP 连接完全解耦
	go func() {
		content, err := fn()
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.running, lessonID)
		task.FinishedAt = nowPtr()
		if err != nil {
			task.Status = TaskError
			task.Error = err.Error()
			return
		}
		task.Status = TaskDone
		task.Content = content
	}()
	return task.ID
}

// Get 查询任务
func (s *TaskService) Get(id string) (*Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	return t, ok
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func nowPtr() *time.Time {
	t := time.Now()
	return &t
}
