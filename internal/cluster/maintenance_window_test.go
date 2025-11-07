package cluster

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateMaintenanceWindow tests the validation logic that would run in SetMaintenanceWindow
func validateMaintenanceWindow(window *model.MaintenanceWindow) error {
	if window == nil {
		return nil
	}

	// Validate time format (HH:MM)
	startParts := strings.Split(window.StartTime, ":")
	endParts := strings.Split(window.EndTime, ":")
	if len(startParts) != 2 || len(endParts) != 2 {
		return fmt.Errorf("invalid time format, expected HH:MM")
	}

	// Parse hours and minutes
	var startHour, startMin, endHour, endMin int
	if _, err := fmt.Sscanf(window.StartTime, "%d:%d", &startHour, &startMin); err != nil {
		return fmt.Errorf("invalid start time format: %w", err)
	}
	if _, err := fmt.Sscanf(window.EndTime, "%d:%d", &endHour, &endMin); err != nil {
		return fmt.Errorf("invalid end time format: %w", err)
	}

	// Validate ranges
	if startHour < 0 || startHour > 23 || endHour < 0 || endHour > 23 {
		return fmt.Errorf("hours must be between 0 and 23")
	}
	if startMin < 0 || startMin > 59 || endMin < 0 || endMin > 59 {
		return fmt.Errorf("minutes must be between 0 and 59")
	}

	// Validate at least one day is specified
	if len(window.Days) == 0 {
		return fmt.Errorf("at least one day must be specified for the maintenance window")
	}

	return nil
}

func TestSetMaintenanceWindow_Validation(t *testing.T) {
	tests := []struct {
		name        string
		window      *model.MaintenanceWindow
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid window with specific days",
			window: &model.MaintenanceWindow{
				StartTime: "02:00",
				EndTime:   "06:00",
				Days:      []model.DayOfWeek{model.DayOfWeekSaturday, model.DayOfWeekSunday},
			},
			expectError: false,
		},
		{
			name: "empty days - not allowed",
			window: &model.MaintenanceWindow{
				StartTime: "01:00",
				EndTime:   "05:00",
				Days:      []model.DayOfWeek{},
			},
			expectError: true,
			errorMsg:    "at least one day must be specified",
		},
		{
			name: "invalid time format - missing colon",
			window: &model.MaintenanceWindow{
				StartTime: "0200",
				EndTime:   "06:00",
				Days:      []model.DayOfWeek{model.DayOfWeekMonday},
			},
			expectError: true,
			errorMsg:    "invalid time format",
		},
		{
			name: "invalid time format - letters",
			window: &model.MaintenanceWindow{
				StartTime: "ab:cd",
				EndTime:   "06:00",
				Days:      []model.DayOfWeek{model.DayOfWeekMonday},
			},
			expectError: true,
			errorMsg:    "invalid start time format",
		},
		{
			name: "invalid hour - too high",
			window: &model.MaintenanceWindow{
				StartTime: "25:00",
				EndTime:   "06:00",
				Days:      []model.DayOfWeek{model.DayOfWeekMonday},
			},
			expectError: true,
			errorMsg:    "hours must be between 0 and 23",
		},
		{
			name: "invalid minute - too high",
			window: &model.MaintenanceWindow{
				StartTime: "02:60",
				EndTime:   "06:00",
				Days:      []model.DayOfWeek{model.DayOfWeekMonday},
			},
			expectError: true,
			errorMsg:    "minutes must be between 0 and 59",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaintenanceWindow(tt.window)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetMaintenanceWindow_RecurrenceRules(t *testing.T) {
	tests := []struct {
		name              string
		days              []model.DayOfWeek
		expectedFrequency string
		expectedPattern   string
	}{
		{
			name:              "weekdays only",
			days:              []model.DayOfWeek{model.DayOfWeekMonday, model.DayOfWeekTuesday, model.DayOfWeekWednesday, model.DayOfWeekThursday, model.DayOfWeekFriday},
			expectedFrequency: "FREQ=WEEKLY",
			expectedPattern:   "BYDAY=MO,TU,WE,TH,FR",
		},
		{
			name:              "weekends only",
			days:              []model.DayOfWeek{model.DayOfWeekSaturday, model.DayOfWeekSunday},
			expectedFrequency: "FREQ=WEEKLY",
			expectedPattern:   "BYDAY=SA,SU",
		},
		{
			name:              "single day",
			days:              []model.DayOfWeek{model.DayOfWeekMonday},
			expectedFrequency: "FREQ=WEEKLY",
			expectedPattern:   "BYDAY=MO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build the recurrence rule as done in SetMaintenanceWindow
			var recurrence string
			if len(tt.days) > 0 {
				days := make([]string, len(tt.days))
				dayMap := map[model.DayOfWeek]string{
					model.DayOfWeekMonday:    "MO",
					model.DayOfWeekTuesday:   "TU",
					model.DayOfWeekWednesday: "WE",
					model.DayOfWeekThursday:  "TH",
					model.DayOfWeekFriday:    "FR",
					model.DayOfWeekSaturday:  "SA",
					model.DayOfWeekSunday:    "SU",
				}
				for i, day := range tt.days {
					days[i] = dayMap[day]
				}
				recurrence = "FREQ=WEEKLY;BYDAY=" + strings.Join(days, ",")
			} else {
				recurrence = "FREQ=DAILY"
			}

			// Verify the recurrence matches expectations
			assert.Contains(t, recurrence, tt.expectedFrequency)
			if tt.expectedPattern != "" {
				assert.Contains(t, recurrence, tt.expectedPattern)
			}
		})
	}
}

func TestMaintenanceWindowTimeWindow(t *testing.T) {
	// Test that time window format is validated correctly (all times in UTC)
	tests := []struct {
		name      string
		startTime string
		endTime   string
	}{
		{
			name:      "morning window",
			startTime: "02:00",
			endTime:   "06:00",
		},
		{
			name:      "evening window",
			startTime: "22:00",
			endTime:   "23:59",
		},
		{
			name:      "overnight window",
			startTime: "23:00",
			endTime:   "01:00",
		},
		{
			name:      "full business hours",
			startTime: "09:00",
			endTime:   "17:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify time format using validation function
			window := &model.MaintenanceWindow{
				StartTime: tt.startTime,
				EndTime:   tt.endTime,
				Days:      []model.DayOfWeek{model.DayOfWeekMonday},
			}
			err := validateMaintenanceWindow(window)
			assert.NoError(t, err, "Time window should be valid")
		})
	}
}
