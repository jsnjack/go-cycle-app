package cmd

import (
	"fmt"
	"testing"
	"time"
)

func Test_renderDescription_simple(t *testing.T) {
	year := time.Now().Year()
	daysLeft := int(time.Until(time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)).Hours()/24 - 1)
	desc, err := renderDescription(1000, 500, 10, "", "")
	if err != nil {
		t.Error(err)
		return
	}

	expected := fmt.Sprintf(`+1.00%% towards the goal!
0.50 of 1.00 km (50.00%%) in %d
0.50 km and %d days remains`, year, daysLeft)

	if desc != expected {
		fmt.Printf("Expected text: %q\n", expected)
		fmt.Printf("  Actual text: %q\n", desc)
		t.Fail()
	}
}

func Test_renderDescription_with_signature(t *testing.T) {
	year := time.Now().Year()
	daysLeft := int(time.Until(time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)).Hours()/24 - 1)
	desc, err := renderDescription(1000, 500, 10, "", "-- app")
	if err != nil {
		t.Error(err)
		return
	}

	expected := fmt.Sprintf(`+1.00%% towards the goal!
0.50 of 1.00 km (50.00%%) in %d
0.50 km and %d days remains
-- app`, year, daysLeft)

	if desc != expected {
		fmt.Printf("Expected text: %q\n", expected)
		fmt.Printf("  Actual text: %q\n", desc)
		t.Fail()
	}
}

func Test_renderDescription_with_description(t *testing.T) {
	year := time.Now().Year()
	daysLeft := int(time.Until(time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)).Hours()/24 - 1)
	desc, err := renderDescription(1000, 500, 10, "other app", "-- app")
	if err != nil {
		t.Error(err)
		return
	}

	expected := fmt.Sprintf(`other app
+1.00%% towards the goal!
0.50 of 1.00 km (50.00%%) in %d
0.50 km and %d days remains
-- app`, year, daysLeft)

	if desc != expected {
		fmt.Printf("Expected text: %q\n", expected)
		fmt.Printf("  Actual text: %q\n", desc)
		t.Fail()
	}
}

func Test_renderDescription_over(t *testing.T) {
	year := time.Now().Year()
	daysLeft := int(time.Until(time.Date(year+1, time.January, 1, 0, 0, 0, 0, time.UTC)).Hours()/24 - 1)
	desc, err := renderDescription(1000000, 1100000, 150000, "", "")
	if err != nil {
		t.Error(err)
		return
	}

	expected := fmt.Sprintf(`🏆 110.00%% of the goal!
1100.00 of 1000.00 km in %d
%d days remains`, year, daysLeft)

	if desc != expected {
		fmt.Printf("Expected text: %q\n", expected)
		fmt.Printf("  Actual text: %q\n", desc)
		t.Fail()
	}
}
