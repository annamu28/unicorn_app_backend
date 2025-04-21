package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"unicorn_app_backend/models"

	"github.com/gin-gonic/gin"
)

type TestHandler struct {
	db *sql.DB
}

func NewTestHandler(db *sql.DB) *TestHandler {
	return &TestHandler{db: db}
}

func (h *TestHandler) CreateTest(c *gin.Context) {
	// Get the user ID from the context
	userID := c.GetInt("userID")

	// Check if user is Admin
	var isAdmin bool
	err := h.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.role = 'Admin'
		)
	`, userID).Scan(&isAdmin)

	if err != nil {
		log.Printf("Error checking admin status: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify permissions"})
		return
	}

	if !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only Admin users can create tests"})
		return
	}

	// Parse request
	var req models.CreateTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Create test
	var testID int
	err = tx.QueryRow(`
		INSERT INTO tests (lesson_id, title, reward_details, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		RETURNING id
	`, req.LessonID, req.Title, req.RewardDetails).Scan(&testID)

	if err != nil {
		log.Printf("Error creating test: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create test"})
		return
	}

	// Create questions
	for _, q := range req.Questions {
		var questionID int
		err = tx.QueryRow(`
			INSERT INTO questions (test_id, question, question_type)
			VALUES ($1, $2, $3)
			RETURNING id
		`, testID, q.Question, q.QuestionType).Scan(&questionID)

		if err != nil {
			log.Printf("Error creating question: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create question"})
			return
		}

		// Create answers for this question
		for _, a := range q.Answers {
			_, err = tx.Exec(`
				INSERT INTO answers (question_id, answer, is_correct, min_value, max_value)
				VALUES ($1, $2, $3, $4, $5)
			`, questionID, a.Answer, a.IsCorrect, a.MinValue, a.MaxValue)

			if err != nil {
				log.Printf("Error creating answer: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create answer"})
				return
			}
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save test"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Test created successfully",
		"test_id": testID,
	})
}

func (h *TestHandler) SubmitTestAttempt(c *gin.Context) {
	userID := c.GetInt("userID")

	var attempt models.TestAttempt
	if err := c.ShouldBindJSON(&attempt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Get test reward details
	var testRewardDetails string
	err = tx.QueryRow(
		"SELECT reward_details FROM tests WHERE id = $1",
		attempt.TestID,
	).Scan(&testRewardDetails)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Test not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get test details"})
		return
	}

	// Insert test attempt
	var attemptID int
	err = tx.QueryRow(
		"INSERT INTO test_attempts (test_id, user_id, score, completed_at) VALUES ($1, $2, $3, CURRENT_DATE) RETURNING id",
		attempt.TestID, userID, attempt.Score,
	).Scan(&attemptID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create test attempt"})
		return
	}

	// Insert user answers
	for _, answer := range attempt.UserAnswers {
		_, err = tx.Exec(
			`INSERT INTO user_answers (attempt_id, question_id, answer_id, answer_number, answer_text, completed_at) 
			VALUES ($1, $2, $3, $4, $5, CURRENT_DATE)`,
			attemptID, answer.QuestionID, answer.AnswerID, answer.AnswerNumber, answer.AnswerText,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user answers"})
			return
		}
	}

	// Determine if the student gets the reward (e.g., if they score 60% or higher)
	var rewardDetails string
	if attempt.Score >= 60 {
		rewardDetails = testRewardDetails
	} else {
		rewardDetails = "Keep practicing! Score 60% or higher to earn the reward."
	}

	// Insert reward
	var rewardID int
	err = tx.QueryRow(
		"INSERT INTO rewards (attempt_id, reward_details, completed_at) VALUES ($1, $2, CURRENT_DATE) RETURNING id",
		attemptID, rewardDetails,
	).Scan(&rewardID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reward"})
		return
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Return attempt with reward
	response := gin.H{
		"attempt_id":     attemptID,
		"score":          attempt.Score,
		"reward_details": rewardDetails,
		"earned_reward":  attempt.Score >= 60,
	}
	c.JSON(http.StatusCreated, response)
}

func (h *TestHandler) GetUserRewards(c *gin.Context) {
	userID := c.GetInt("userID") // Get from auth middleware

	rows, err := h.db.Query(`
		SELECT r.id, r.reward_details, t.title, ta.score, ta.completed_at
		FROM rewards r
		JOIN test_attempts ta ON r.attempt_id = ta.id
		JOIN tests t ON ta.test_id = t.id
		WHERE ta.user_id = $1
		ORDER BY ta.completed_at DESC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rewards"})
		return
	}
	defer rows.Close()

	var rewards []gin.H
	for rows.Next() {
		var (
			id            int
			rewardDetails string
			testTitle     string
			score         int
			completedAt   string
		)
		if err := rows.Scan(&id, &rewardDetails, &testTitle, &score, &completedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan rewards"})
			return
		}
		rewards = append(rewards, gin.H{
			"id":             id,
			"reward_details": rewardDetails,
			"test_title":     testTitle,
			"score":          score,
			"completed_at":   completedAt,
		})
	}

	c.JSON(http.StatusOK, rewards)
}

func (h *TestHandler) CreateReward(c *gin.Context) {
	// Check if user has required role
	userRole := c.GetString("userRole")
	if userRole != "Admin" && userRole != "Head Unicorn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	var reward models.Reward
	if err := c.ShouldBindJSON(&reward); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify that the attempt exists and get user information
	var (
		userID    int
		testTitle string
		score     int
	)
	err := h.db.QueryRow(`
		SELECT ta.user_id, t.title, ta.score 
		FROM test_attempts ta
		JOIN tests t ON ta.test_id = t.id
		WHERE ta.id = $1`,
		reward.AttemptID,
	).Scan(&userID, &testTitle, &score)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Test attempt not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify test attempt"})
		return
	}

	// Check if reward already exists for this attempt
	var existingRewardID int
	err = h.db.QueryRow("SELECT id FROM rewards WHERE attempt_id = $1", reward.AttemptID).Scan(&existingRewardID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Reward already exists for this attempt"})
		return
	} else if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing reward"})
		return
	}

	// Insert the new reward
	var rewardID int
	err = h.db.QueryRow(
		"INSERT INTO rewards (attempt_id, reward_details, completed_at) VALUES ($1, $2, CURRENT_DATE) RETURNING id",
		reward.AttemptID, reward.RewardDetails,
	).Scan(&rewardID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reward"})
		return
	}

	// Return the created reward with additional context
	response := gin.H{
		"id":             rewardID,
		"attempt_id":     reward.AttemptID,
		"reward_details": reward.RewardDetails,
		"test_title":     testTitle,
		"score":          score,
		"user_id":        userID,
	}
	c.JSON(http.StatusCreated, response)
}

func (h *TestHandler) UpdateReward(c *gin.Context) {
	// Check if user has required role
	userRole := c.GetString("userRole")
	if userRole != "Admin" && userRole != "Head Unicorn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	rewardID := c.Param("id")
	var reward models.Reward
	if err := c.ShouldBindJSON(&reward); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update the reward
	result, err := h.db.Exec(
		"UPDATE rewards SET reward_details = $1 WHERE id = $2",
		reward.RewardDetails, rewardID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reward"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify update"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reward not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":             rewardID,
		"reward_details": reward.RewardDetails,
		"message":        "Reward updated successfully",
	})
}

func (h *TestHandler) GetTests(c *gin.Context) {
	// Optional query parameters for filtering
	lessonID := c.Query("lesson_id")

	query := `
		SELECT t.id, t.lesson_id, t.title, t.reward_details, t.created_at,
		       l.title as lesson_title
		FROM tests t
		LEFT JOIN lessons l ON t.lesson_id = l.id
		WHERE 1=1
	`
	args := []interface{}{}

	if lessonID != "" {
		query += " AND t.lesson_id = $1"
		args = append(args, lessonID)
	}

	query += " ORDER BY t.created_at DESC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tests"})
		return
	}
	defer rows.Close()

	var tests []gin.H
	for rows.Next() {
		var (
			id            int
			lessonID      sql.NullInt64
			title         string
			rewardDetails string
			createdAt     string
			lessonTitle   sql.NullString
		)

		if err := rows.Scan(&id, &lessonID, &title, &rewardDetails, &createdAt, &lessonTitle); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan test"})
			return
		}

		test := gin.H{
			"id":             id,
			"title":          title,
			"reward_details": rewardDetails,
			"created_at":     createdAt,
		}

		if lessonID.Valid {
			test["lesson_id"] = lessonID.Int64
			test["lesson_title"] = lessonTitle.String
		}

		tests = append(tests, test)
	}

	c.JSON(http.StatusOK, tests)
}

func (h *TestHandler) GetTestByID(c *gin.Context) {
	testID := c.Param("id")

	// Get test details
	var test struct {
		ID            int            `json:"id"`
		LessonID      sql.NullInt64  `json:"lesson_id,omitempty"`
		Title         string         `json:"title"`
		RewardDetails string         `json:"reward_details"`
		CreatedAt     string         `json:"created_at"`
		LessonTitle   sql.NullString `json:"lesson_title,omitempty"`
		Questions     []struct {
			ID           int    `json:"id"`
			Question     string `json:"question"`
			QuestionType string `json:"question_type"`
			Answers      []struct {
				ID        int    `json:"id"`
				Answer    string `json:"answer"`
				IsCorrect bool   `json:"is_correct"`
				MinValue  *int   `json:"min_value,omitempty"`
				MaxValue  *int   `json:"max_value,omitempty"`
			} `json:"answers"`
		} `json:"questions"`
	}

	// Get test and lesson information
	err := h.db.QueryRow(`
		SELECT t.id, t.lesson_id, t.title, t.reward_details, t.created_at, l.title
		FROM tests t
		LEFT JOIN lessons l ON t.lesson_id = l.id
		WHERE t.id = $1
	`, testID).Scan(&test.ID, &test.LessonID, &test.Title, &test.RewardDetails, &test.CreatedAt, &test.LessonTitle)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Test not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch test"})
		return
	}

	// Get questions
	questionRows, err := h.db.Query(`
		SELECT id, question, question_type
		FROM questions
		WHERE test_id = $1
		ORDER BY id
	`, testID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch questions"})
		return
	}
	defer questionRows.Close()

	// Process questions and their answers
	for questionRows.Next() {
		var q struct {
			ID           int    `json:"id"`
			Question     string `json:"question"`
			QuestionType string `json:"question_type"`
			Answers      []struct {
				ID        int    `json:"id"`
				Answer    string `json:"answer"`
				IsCorrect bool   `json:"is_correct"`
				MinValue  *int   `json:"min_value,omitempty"`
				MaxValue  *int   `json:"max_value,omitempty"`
			} `json:"answers"`
		}

		err := questionRows.Scan(&q.ID, &q.Question, &q.QuestionType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan question"})
			return
		}

		// Get answers for this question
		answerRows, err := h.db.Query(`
			SELECT id, answer, is_correct, min_value, max_value
			FROM answers
			WHERE question_id = $1
			ORDER BY id
		`, q.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch answers"})
			return
		}
		defer answerRows.Close()

		for answerRows.Next() {
			var a struct {
				ID        int    `json:"id"`
				Answer    string `json:"answer"`
				IsCorrect bool   `json:"is_correct"`
				MinValue  *int   `json:"min_value,omitempty"`
				MaxValue  *int   `json:"max_value,omitempty"`
			}

			if err := answerRows.Scan(&a.ID, &a.Answer, &a.IsCorrect, &a.MinValue, &a.MaxValue); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan answer"})
				return
			}

			q.Answers = append(q.Answers, a)
		}

		test.Questions = append(test.Questions, q)
	}

	c.JSON(http.StatusOK, test)
}
