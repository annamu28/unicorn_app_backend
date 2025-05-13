package models

type Test struct {
	ID            int        `json:"id"`
	LessonID      *int       `json:"lesson_id"` // Nullable since it's optional in schema
	Title         string     `json:"title"`
	Questions     []Question `json:"questions"`
	RewardDetails string     `json:"reward_details"` // The reward for completing this test
}

type CreateTestRequest struct {
	LessonID      int        `json:"lesson_id"`
	Title         string     `json:"title" binding:"required"`
	RewardDetails string     `json:"reward_details"`
	Questions     []Question `json:"questions" binding:"required"`
}

type Question struct {
	ID           int      `json:"id"`
	TestID       int      `json:"test_id"`
	Question     string   `json:"question" binding:"required"`
	QuestionType string   `json:"question_type" binding:"required"`
	Answers      []Answer `json:"answers" binding:"required"`
}

type Answer struct {
	ID         int    `json:"id"`
	QuestionID int    `json:"question_id"`
	Answer     string `json:"answer" binding:"required"`
	IsCorrect  bool   `json:"is_correct"`
	MinValue   *int   `json:"min_value,omitempty"`
	MaxValue   *int   `json:"max_value,omitempty"`
}

type TestAttempt struct {
	ID          int          `json:"id"`
	TestID      int          `json:"test_id" binding:"required"`
	UserID      int          `json:"user_id"`
	Score       int          `json:"score" binding:"required"`
	UserAnswers []UserAnswer `json:"user_answers" binding:"required"`
}

type UserAnswer struct {
	QuestionID   int     `json:"question_id" binding:"required"`
	AnswerID     int     `json:"answer_id"`
	AnswerNumber *int    `json:"answer_number,omitempty"`
	AnswerText   *string `json:"answer_text,omitempty"`
}

type Reward struct {
	ID            int    `json:"id"`
	AttemptID     int    `json:"attempt_id" binding:"required"`
	RewardDetails string `json:"reward_details" binding:"required"`
}

type RewardCatalog struct {
	ID          int    `json:"id"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
	Points      int    `json:"points" binding:"required"`
	Type        string `json:"type" binding:"required"`
	CreatedAt   string `json:"created_at"`
}

type CreateRewardCatalogRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
	Points      int    `json:"points" binding:"required"`
	Type        string `json:"type" binding:"required"`
}
