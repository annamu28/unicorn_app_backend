package handlers

import (
	"database/sql"
	"log"
	"net/http"
	"strings"
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

	// Check if user has already attempted this test
	var existingAttemptID int
	err := h.db.QueryRow(
		"SELECT id FROM test_attempts WHERE test_id = $1 AND user_id = $2",
		attempt.TestID, userID,
	).Scan(&existingAttemptID)
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "You have already taken this test"})
		return
	} else if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing attempt"})
		return
	}

	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Insert test attempt
	var attemptID int
	err = tx.QueryRow(
		"INSERT INTO test_attempts (test_id, user_id, score, completed_at) VALUES ($1, $2, $3, CURRENT_DATE) RETURNING id",
		attempt.TestID, userID, attempt.Score,
	).Scan(&attemptID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			c.JSON(http.StatusConflict, gin.H{"error": "You have already taken this test"})
			return
		}
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

	// Determine which reward to give based on score
	var rewardCatalogID int
	var rewardDetails string
	if attempt.Score >= 90 {
		err = tx.QueryRow("SELECT id FROM rewards_catalog WHERE name = 'Test Master'").Scan(&rewardCatalogID)
		rewardDetails = "Congratulations! You've achieved Test Master status!"
	} else if attempt.Score >= 80 {
		err = tx.QueryRow("SELECT id FROM rewards_catalog WHERE name = 'Quick Learner'").Scan(&rewardCatalogID)
		rewardDetails = "Great job! You're a Quick Learner!"
	} else if attempt.Score >= 70 {
		err = tx.QueryRow("SELECT id FROM rewards_catalog WHERE name = 'Good Progress'").Scan(&rewardCatalogID)
		rewardDetails = "Well done! You're making Good Progress!"
	} else if attempt.Score >= 20 {
		err = tx.QueryRow("SELECT id FROM rewards_catalog WHERE name = 'Test Completion'").Scan(&rewardCatalogID)
		rewardDetails = "Good work! You've completed the test!"
	} else {
		rewardDetails = "Keep practicing! Score 20% or higher to earn a reward."
	}

	// If score is high enough, create the reward
	if attempt.Score >= 20 {
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get reward catalog"})
			return
		}

		var rewardID int
		err = tx.QueryRow(
			"INSERT INTO rewards (attempt_id, reward_catalog_id, reward_details, created_at) VALUES ($1, $2, $3, CURRENT_DATE) RETURNING id",
			attemptID, rewardCatalogID, rewardDetails,
		).Scan(&rewardID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reward"})
			return
		}
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
		"earned_reward":  attempt.Score >= 20,
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
			completedAt   sql.NullTime
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
			"completed_at":   completedAt.Time.Format("2006-01-02 15:04"),
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
			createdAt     sql.NullTime
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
			"created_at":     createdAt.Time.Format("2006-01-02 15:04"),
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
	var createdAt sql.NullTime
	err := h.db.QueryRow(`
		SELECT t.id, t.lesson_id, t.title, t.reward_details, t.created_at, l.title
		FROM tests t
		LEFT JOIN lessons l ON t.lesson_id = l.id
		WHERE t.id = $1
	`, testID).Scan(&test.ID, &test.LessonID, &test.Title, &test.RewardDetails, &createdAt, &test.LessonTitle)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Test not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch test"})
		return
	}

	// Format the created_at time
	if createdAt.Valid {
		test.CreatedAt = createdAt.Time.Format("2006-01-02 15:04")
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

func (h *TestHandler) GetRewardsCatalog(c *gin.Context) {
	rows, err := h.db.Query(`
		SELECT id, name, description, points, type, created_at
		FROM rewards_catalog
		ORDER BY points DESC, name ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rewards catalog"})
		return
	}
	defer rows.Close()

	var rewards []models.RewardCatalog
	for rows.Next() {
		var reward models.RewardCatalog
		var createdAt sql.NullTime
		if err := rows.Scan(&reward.ID, &reward.Name, &reward.Description, &reward.Points, &reward.Type, &createdAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan reward"})
			return
		}
		if createdAt.Valid {
			reward.CreatedAt = createdAt.Time.Format("2006-01-02 15:04")
		}
		rewards = append(rewards, reward)
	}

	c.JSON(http.StatusOK, rewards)
}

func (h *TestHandler) CreateRewardCatalog(c *gin.Context) {
	// Check if user has required role
	userRole := c.GetString("userRole")
	if userRole != "Admin" && userRole != "Head Unicorn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	var req models.CreateRewardCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var rewardID int
	err := h.db.QueryRow(`
		INSERT INTO rewards_catalog (name, description, points, type, created_at)
		VALUES ($1, $2, $3, $4, CURRENT_DATE)
		RETURNING id`,
		req.Name, req.Description, req.Points, req.Type,
	).Scan(&rewardID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create reward"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          rewardID,
		"name":        req.Name,
		"description": req.Description,
		"points":      req.Points,
		"type":        req.Type,
	})
}

func (h *TestHandler) UpdateRewardCatalog(c *gin.Context) {
	// Check if user has required role
	userRole := c.GetString("userRole")
	if userRole != "Admin" && userRole != "Head Unicorn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	rewardID := c.Param("id")
	var req models.CreateRewardCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.db.Exec(`
		UPDATE rewards_catalog
		SET name = $1, description = $2, points = $3, type = $4
		WHERE id = $5`,
		req.Name, req.Description, req.Points, req.Type, rewardID,
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
		"id":          rewardID,
		"name":        req.Name,
		"description": req.Description,
		"points":      req.Points,
		"type":        req.Type,
	})
}

func (h *TestHandler) DeleteRewardCatalog(c *gin.Context) {
	// Check if user has required role
	userRole := c.GetString("userRole")
	if userRole != "Admin" && userRole != "Head Unicorn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	rewardID := c.Param("id")

	// Check if reward is being used
	var count int
	err := h.db.QueryRow(`
		SELECT COUNT(*) FROM rewards WHERE reward_catalog_id = $1`,
		rewardID,
	).Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check reward usage"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete reward that is in use"})
		return
	}

	result, err := h.db.Exec("DELETE FROM rewards_catalog WHERE id = $1", rewardID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete reward"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify deletion"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Reward not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reward deleted successfully"})
}

// ActivateTestInChatboard activates a test in a specific chatboard
func (h *TestHandler) ActivateTestInChatboard(c *gin.Context) {
	// Check if user has required role
	userRole := c.GetString("userRole")
	if userRole != "Admin" && userRole != "Head Unicorn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	var req struct {
		ChatboardID int `json:"chatboard_id" binding:"required"`
		TestID      int `json:"test_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if chatboard exists
	var chatboardExists bool
	err := h.db.QueryRow("SELECT EXISTS(SELECT 1 FROM chatboards WHERE id = $1)", req.ChatboardID).Scan(&chatboardExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify chatboard"})
		return
	}
	if !chatboardExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chatboard not found"})
		return
	}

	// Check if test exists
	var testExists bool
	err = h.db.QueryRow("SELECT EXISTS(SELECT 1 FROM tests WHERE id = $1)", req.TestID).Scan(&testExists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify test"})
		return
	}
	if !testExists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test not found"})
		return
	}

	// Activate test in chatboard
	var id int
	err = h.db.QueryRow(`
		INSERT INTO chatboard_tests (chatboard_id, test_id, is_active)
		VALUES ($1, $2, true)
		ON CONFLICT (chatboard_id, test_id) 
		DO UPDATE SET is_active = true
		RETURNING id
	`, req.ChatboardID, req.TestID).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate test in chatboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test activated in chatboard successfully",
		"id":      id,
	})
}

// DeactivateTestInChatboard deactivates a test in a specific chatboard
func (h *TestHandler) DeactivateTestInChatboard(c *gin.Context) {
	// Check if user has required role
	userRole := c.GetString("userRole")
	if userRole != "Admin" && userRole != "Head Unicorn" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		return
	}

	var req struct {
		ChatboardID int `json:"chatboard_id" binding:"required"`
		TestID      int `json:"test_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.db.Exec(`
		UPDATE chatboard_tests 
		SET is_active = false 
		WHERE chatboard_id = $1 AND test_id = $2
	`, req.ChatboardID, req.TestID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deactivate test in chatboard"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify deactivation"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Test not found in chatboard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Test deactivated in chatboard successfully",
	})
}

// GetChatboardTests gets all tests for a specific chatboard
func (h *TestHandler) GetChatboardTests(c *gin.Context) {
	chatboardID := c.Param("chatboard_id")

	rows, err := h.db.Query(`
		SELECT t.id, t.title, t.reward_details, t.created_at, ct.is_active
		FROM tests t
		JOIN chatboard_tests ct ON t.id = ct.test_id
		WHERE ct.chatboard_id = $1
		ORDER BY t.created_at DESC
	`, chatboardID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chatboard tests"})
		return
	}
	defer rows.Close()

	var tests []gin.H
	for rows.Next() {
		var (
			id            int
			title         string
			rewardDetails string
			createdAt     sql.NullTime
			isActive      bool
		)

		if err := rows.Scan(&id, &title, &rewardDetails, &createdAt, &isActive); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan test"})
			return
		}

		test := gin.H{
			"id":             id,
			"title":          title,
			"reward_details": rewardDetails,
			"is_active":      isActive,
		}

		if createdAt.Valid {
			test["created_at"] = createdAt.Time.Format("2006-01-02 15:04")
		}

		tests = append(tests, test)
	}

	c.JSON(http.StatusOK, tests)
}
