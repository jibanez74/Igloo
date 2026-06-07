package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"igloo/cmd/internal/database"
)

func setupNotificationTestApp(t *testing.T) *Application {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	schemaBeforeFTS, _, ok := strings.Cut(SQL, "-- FTS5 virtual tables")
	if !ok {
		t.Fatal("expected schema to contain FTS marker")
	}

	_, err = db.Exec(schemaBeforeFTS)
	if err != nil {
		t.Fatalf("create pre-FTS schema: %v", err)
	}

	_, notificationsSchema, ok := strings.Cut(SQL, "-- notifications")
	if !ok {
		t.Fatal("expected schema to contain notifications marker")
	}

	_, err = db.Exec("-- notifications" + notificationsSchema)
	if err != nil {
		t.Fatalf("create notifications schema: %v", err)
	}

	return &Application{
		DB:      db,
		Queries: database.New(db),
	}
}

func notificationUserID(userID int64) sql.NullInt64 {
	return sql.NullInt64{Int64: userID, Valid: true}
}

func expectVisibleNotificationStates(t *testing.T, got []database.ListVisibleNotificationsRow, want map[string]bool) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("expected %d notifications, got %d: %#v", len(want), len(got), got)
	}

	remaining := make(map[string]bool, len(want))
	for message, isRead := range want {
		remaining[message] = isRead
	}

	for _, notification := range got {
		expectedRead, ok := remaining[notification.Message]
		if !ok {
			t.Fatalf("unexpected notification %q in %#v", notification.Message, got)
		}
		if notification.IsRead != expectedRead {
			t.Fatalf("notification %q is_read = %t, want %t", notification.Message, notification.IsRead, expectedRead)
		}
		delete(remaining, notification.Message)
	}

	if len(remaining) > 0 {
		t.Fatalf("missing notifications: %#v", remaining)
	}
}

func TestNotificationsSchemaConstraints(t *testing.T) {
	app := setupNotificationTestApp(t)
	creator := createTestUser(t, app, "Creator User", "creator@example.com", false)
	target := createTestUser(t, app, "Target User", "target@example.com", false)

	_, err := app.DB.Exec(`
		INSERT INTO notifications (title, message)
		VALUES ('other', 'missing creator')
	`)
	if err == nil {
		t.Fatal("expected missing created_by_user_id to fail")
	}

	_, err = app.DB.Exec(`
		INSERT INTO notifications (created_by_user_id, title, message)
		VALUES (99999, 'other', 'missing creator user')
	`)
	if err == nil {
		t.Fatal("expected missing created_by_user_id foreign key to fail")
	}

	_, err = app.DB.Exec(`
		INSERT INTO notifications (created_by_user_id, title, message)
		VALUES (?, 'other', 'valid notification')
	`, creator.ID)
	if err != nil {
		t.Fatalf("insert valid notification: %v", err)
	}

	var createdByUserID int64
	err = app.DB.QueryRow(`
		SELECT created_by_user_id
		FROM notifications
		WHERE message = 'valid notification'
	`).Scan(&createdByUserID)
	if err != nil {
		t.Fatalf("select created_by_user_id: %v", err)
	}
	if createdByUserID != creator.ID {
		t.Fatalf("expected created_by_user_id %d, got %d", creator.ID, createdByUserID)
	}

	_, err = app.DB.Exec(`
		INSERT INTO notifications (created_by_user_id, title, message)
		VALUES (?, 'invalid', 'bad title')
	`, creator.ID)
	if err == nil {
		t.Fatal("expected invalid notification title to fail")
	}

	_, err = app.DB.Exec(`
		INSERT INTO notifications (created_by_user_id, user_id, title, message)
		VALUES (?, 99999, 'other', 'missing target user')
	`, creator.ID)
	if err == nil {
		t.Fatal("expected missing user_id foreign key to fail")
	}

	_, err = app.DB.Exec(`
		INSERT INTO notifications (created_by_user_id, user_id, title, message, is_admin)
		VALUES (?, ?, 'other', 'mixed target', true)
	`, creator.ID, target.ID)
	if err == nil {
		t.Fatal("expected user_id plus is_admin=true to fail")
	}
}

func TestNotificationsDefaultIsAdmin(t *testing.T) {
	app := setupNotificationTestApp(t)
	creator := createTestUser(t, app, "Creator User", "creator@example.com", false)

	_, err := app.DB.Exec(`
		INSERT INTO notifications (created_by_user_id, title, message)
		VALUES (?, 'other', 'uses default is_admin')
	`, creator.ID)
	if err != nil {
		t.Fatalf("insert notification with default is_admin: %v", err)
	}

	var isAdmin bool
	err = app.DB.QueryRow(`
		SELECT is_admin
		FROM notifications
		WHERE message = 'uses default is_admin'
	`).Scan(&isAdmin)
	if err != nil {
		t.Fatalf("select default is_admin: %v", err)
	}
	if isAdmin {
		t.Fatal("expected is_admin to default to false")
	}
}

func TestNotificationCreatorDeleteCascades(t *testing.T) {
	app := setupNotificationTestApp(t)
	ctx := context.Background()

	creator := createTestUser(t, app, "Creator User", "creator@example.com", false)
	target := createTestUser(t, app, "Target User", "target@example.com", false)

	notification, err := app.Queries.CreateNotification(ctx, database.CreateNotificationParams{
		CreatedByUserID: creator.ID,
		UserID:          notificationUserID(target.ID),
		Title:           "other",
		Message:         "created by deleted user",
	})
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}

	err = app.Queries.DeleteUser(ctx, creator.ID)
	if err != nil {
		t.Fatalf("delete creator: %v", err)
	}

	var count int64
	err = app.DB.QueryRow(`
		SELECT COUNT(*)
		FROM notifications
		WHERE id = ?
	`, notification.ID).Scan(&count)
	if err != nil {
		t.Fatalf("count deleted notification: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected creator deletion to delete notification, got count %d", count)
	}
}

func TestNotificationVisibilityQueries(t *testing.T) {
	app := setupNotificationTestApp(t)
	ctx := context.Background()

	creator := createTestUser(t, app, "Creator User", "creator@example.com", false)
	user := createTestUser(t, app, "Regular User", "regular@example.com", false)
	otherUser := createTestUser(t, app, "Other User", "other@example.com", false)
	admin := createTestUser(t, app, "Admin User", "admin@example.com", true)

	globalNotification, err := app.Queries.CreateNotification(ctx, database.CreateNotificationParams{
		CreatedByUserID: creator.ID,
		Title:           "other",
		Message:         "global",
	})
	if err != nil {
		t.Fatalf("create global notification: %v", err)
	}

	userNotification, err := app.Queries.CreateNotification(ctx, database.CreateNotificationParams{
		CreatedByUserID: creator.ID,
		UserID:          notificationUserID(user.ID),
		Title:           "movie_request",
		Message:         "for regular user",
	})
	if err != nil {
		t.Fatalf("create user notification: %v", err)
	}

	otherUserNotification, err := app.Queries.CreateNotification(ctx, database.CreateNotificationParams{
		CreatedByUserID: creator.ID,
		UserID:          notificationUserID(otherUser.ID),
		Title:           "album_request",
		Message:         "for other user",
	})
	if err != nil {
		t.Fatalf("create other user notification: %v", err)
	}

	adminNotification, err := app.Queries.CreateNotification(ctx, database.CreateNotificationParams{
		CreatedByUserID: creator.ID,
		Title:           "track_request",
		Message:         "for admins",
		IsAdmin:         true,
	})
	if err != nil {
		t.Fatalf("create admin notification: %v", err)
	}

	regularRows, err := app.Queries.ListVisibleNotifications(ctx, database.ListVisibleNotificationsParams{
		UserID:  user.ID,
		IsAdmin: false,
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("list regular user notifications: %v", err)
	}
	expectVisibleNotificationStates(t, regularRows, map[string]bool{
		"global":           false,
		"for regular user": false,
	})

	adminRows, err := app.Queries.ListVisibleNotifications(ctx, database.ListVisibleNotificationsParams{
		UserID:  admin.ID,
		IsAdmin: true,
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("list admin notifications: %v", err)
	}
	expectVisibleNotificationStates(t, adminRows, map[string]bool{
		"global":     false,
		"for admins": false,
	})

	globalVisible, err := app.Queries.GetVisibleNotification(ctx, database.GetVisibleNotificationParams{
		ID:      globalNotification.ID,
		UserID:  user.ID,
		IsAdmin: false,
	})
	if err != nil {
		t.Fatalf("expected regular user to fetch global notification: %v", err)
	}
	if globalVisible.IsRead {
		t.Fatal("expected global notification to be unread by default")
	}

	ownVisible, err := app.Queries.GetVisibleNotification(ctx, database.GetVisibleNotificationParams{
		ID:      userNotification.ID,
		UserID:  user.ID,
		IsAdmin: false,
	})
	if err != nil {
		t.Fatalf("expected regular user to fetch own notification: %v", err)
	}
	if ownVisible.IsRead {
		t.Fatal("expected targeted notification to be unread by default")
	}

	_, err = app.Queries.GetVisibleNotification(ctx, database.GetVisibleNotificationParams{
		ID:      otherUserNotification.ID,
		UserID:  user.ID,
		IsAdmin: false,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected other user's notification to be hidden, got %v", err)
	}

	_, err = app.Queries.GetVisibleNotification(ctx, database.GetVisibleNotificationParams{
		ID:      adminNotification.ID,
		UserID:  user.ID,
		IsAdmin: false,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected admin notification to be hidden from regular user, got %v", err)
	}

	_, err = app.Queries.GetVisibleNotification(ctx, database.GetVisibleNotificationParams{
		ID:      adminNotification.ID,
		UserID:  admin.ID,
		IsAdmin: true,
	})
	if err != nil {
		t.Fatalf("expected admin to fetch admin notification: %v", err)
	}
}

func TestNotificationReadStateQueries(t *testing.T) {
	app := setupNotificationTestApp(t)
	ctx := context.Background()

	creator := createTestUser(t, app, "Creator User", "creator@example.com", false)
	user := createTestUser(t, app, "Regular User", "regular@example.com", false)
	otherUser := createTestUser(t, app, "Other User", "other@example.com", false)
	admin := createTestUser(t, app, "Admin User", "admin@example.com", true)

	globalNotification, err := app.Queries.CreateNotification(ctx, database.CreateNotificationParams{
		CreatedByUserID: creator.ID,
		Title:           "other",
		Message:         "global",
	})
	if err != nil {
		t.Fatalf("create global notification: %v", err)
	}

	adminNotification, err := app.Queries.CreateNotification(ctx, database.CreateNotificationParams{
		CreatedByUserID: creator.ID,
		Title:           "track_request",
		Message:         "for admins",
		IsAdmin:         true,
	})
	if err != nil {
		t.Fatalf("create admin notification: %v", err)
	}

	err = app.Queries.MarkNotificationRead(ctx, database.MarkNotificationReadParams{
		NotificationID: globalNotification.ID,
		UserID:         user.ID,
		IsAdmin:        false,
	})
	if err != nil {
		t.Fatalf("mark global notification read: %v", err)
	}

	err = app.Queries.MarkNotificationRead(ctx, database.MarkNotificationReadParams{
		NotificationID: globalNotification.ID,
		UserID:         user.ID,
		IsAdmin:        false,
	})
	if err != nil {
		t.Fatalf("mark global notification read twice: %v", err)
	}

	err = app.Queries.MarkNotificationRead(ctx, database.MarkNotificationReadParams{
		NotificationID: adminNotification.ID,
		UserID:         user.ID,
		IsAdmin:        false,
	})
	if err != nil {
		t.Fatalf("mark hidden admin notification read should be a no-op, got %v", err)
	}

	err = app.Queries.MarkNotificationRead(ctx, database.MarkNotificationReadParams{
		NotificationID: adminNotification.ID,
		UserID:         admin.ID,
		IsAdmin:        true,
	})
	if err != nil {
		t.Fatalf("mark admin notification read: %v", err)
	}

	userRows, err := app.Queries.ListVisibleNotifications(ctx, database.ListVisibleNotificationsParams{
		UserID:  user.ID,
		IsAdmin: false,
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("list user notifications after read: %v", err)
	}
	expectVisibleNotificationStates(t, userRows, map[string]bool{
		"global": true,
	})

	adminRows, err := app.Queries.ListVisibleNotifications(ctx, database.ListVisibleNotificationsParams{
		UserID:  admin.ID,
		IsAdmin: true,
		Limit:   20,
	})
	if err != nil {
		t.Fatalf("list admin notifications after read: %v", err)
	}
	expectVisibleNotificationStates(t, adminRows, map[string]bool{
		"global":     false,
		"for admins": true,
	})

	userVisible, err := app.Queries.GetVisibleNotification(ctx, database.GetVisibleNotificationParams{
		ID:      globalNotification.ID,
		UserID:  user.ID,
		IsAdmin: false,
	})
	if err != nil {
		t.Fatalf("fetch user read state: %v", err)
	}
	if !userVisible.IsRead {
		t.Fatal("expected user global notification to be marked read")
	}

	adminVisible, err := app.Queries.GetVisibleNotification(ctx, database.GetVisibleNotificationParams{
		ID:      adminNotification.ID,
		UserID:  admin.ID,
		IsAdmin: true,
	})
	if err != nil {
		t.Fatalf("fetch admin read state: %v", err)
	}
	if !adminVisible.IsRead {
		t.Fatal("expected admin notification to be marked read")
	}

	var hiddenReadCount int64
	err = app.DB.QueryRow(`
		SELECT COUNT(*)
		FROM notification_reads
		WHERE notification_id = ?
		  AND user_id = ?
	`, adminNotification.ID, user.ID).Scan(&hiddenReadCount)
	if err != nil {
		t.Fatalf("count hidden read rows: %v", err)
	}
	if hiddenReadCount != 0 {
		t.Fatalf("expected hidden notification read row count 0, got %d", hiddenReadCount)
	}

	var totalReadCount int64
	err = app.DB.QueryRow(`
		SELECT COUNT(*)
		FROM notification_reads
		WHERE notification_id = ?
		  AND user_id = ?
	`, globalNotification.ID, user.ID).Scan(&totalReadCount)
	if err != nil {
		t.Fatalf("count global read rows: %v", err)
	}
	if totalReadCount != 1 {
		t.Fatalf("expected one read row after idempotent mark, got %d", totalReadCount)
	}

	_ = otherUser
}
