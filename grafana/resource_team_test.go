package grafana

import (
	"fmt"
	"strconv"
	"testing"

	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccTeam_basic(t *testing.T) {
	CheckOSSTestsEnabled(t)

	var team gapi.Team

	// TODO: Make parallelizable
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccTeamCheckDestroy(&team),
		Steps: []resource.TestStep{
			{
				Config: testAccTeamConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccTeamCheckExists("grafana_team.test", &team),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "name", "terraform-acc-test",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "email", "teamEmail@example.com",
					),
					resource.TestMatchResourceAttr(
						"grafana_team.test", "id", idRegexp,
					),
				),
			},
			{
				Config: testAccTeamConfig_updateName,
				Check: resource.ComposeTestCheckFunc(
					testAccTeamCheckExists("grafana_team.test", &team),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "name", "terraform-acc-test-update",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "email", "teamEmailUpdate@example.com",
					),
				),
			},
			{
				ResourceName:            "grafana_team.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"ignore_externally_synced_members"},
			},
		},
	})
}

func TestAccTeam_Members(t *testing.T) {
	CheckOSSTestsEnabled(t)

	var team gapi.Team

	// TODO: Make parallelizable
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccTeamCheckDestroy(&team),
		Steps: []resource.TestStep{
			{
				Config: testAccTeamConfig_memberAdd,
				Check: resource.ComposeTestCheckFunc(
					testAccTeamCheckExists("grafana_team.test", &team),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "name", "terraform-acc-test",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "members.#", "2",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "members.0", "test-team-1@example.com",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "members.1", "test-team-2@example.com",
					),
				),
			},
			{
				Config: testAccTeamConfig_memberReorder,
				Check: resource.ComposeTestCheckFunc(
					testAccTeamCheckExists("grafana_team.test", &team),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "name", "terraform-acc-test",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "members.#", "2",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "members.0", "test-team-1@example.com",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "members.1", "test-team-2@example.com",
					),
				),
			},
			// Test the import with members
			{
				ResourceName:            "grafana_team.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"ignore_externally_synced_members"},
			},
			{
				Config: testAccTeamConfig_memberRemove,
				Check: resource.ComposeTestCheckFunc(
					testAccTeamCheckExists("grafana_team.test", &team),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "name", "terraform-acc-test",
					),
					resource.TestCheckResourceAttr(
						"grafana_team.test", "members.#", "0",
					),
				),
			},
		},
	})
}

// Test that deleted users can still be removed as members of a team
func TestAccTeam_RemoveUnexistingMember(t *testing.T) {
	CheckOSSTestsEnabled(t)
	client := testAccProvider.Meta().(*client).gapi

	var team gapi.Team
	var userID int64 = -1

	// TODO: Make parallelizable
	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccTeamCheckDestroy(&team),
		Steps: []resource.TestStep{
			{
				PreConfig: func() {
					// Create user
					user := gapi.User{
						Email:    "user1@grafana.com",
						Login:    "user1@grafana.com",
						Name:     "user1",
						Password: "123456",
					}
					var err error
					userID, err = client.CreateUser(user)
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: testAccTeam_withMember("user1@grafana.com"),
				Check: resource.ComposeTestCheckFunc(
					testAccTeamCheckExists("grafana_team.test", &team),
					resource.TestCheckResourceAttr("grafana_team.test", "members.#", "1"),
					resource.TestCheckResourceAttr("grafana_team.test", "members.0", "user1@grafana.com"),
				),
			},
			{
				PreConfig: func() {
					// Delete the user
					if err := client.DeleteUser(userID); err != nil {
						t.Fatal(err)
					}
				},
				Config: testAccTeamConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccTeamCheckExists("grafana_team.test", &team),
					resource.TestCheckResourceAttr("grafana_team.test", "members.#", "0"),
				),
			},
		},
	})
}

//nolint:unparam // `rn` always receives `"grafana_team.test"`
func testAccTeamCheckExists(rn string, a *gapi.Team) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("resource id not set")
		}
		id, err := strconv.ParseInt(rs.Primary.ID, 10, 64)
		if err != nil {
			return fmt.Errorf("resource id is malformed")
		}

		client := testAccProvider.Meta().(*client).gapi
		team, err := client.Team(id)
		if err != nil {
			return fmt.Errorf("error getting data source: %s", err)
		}

		*a = *team

		return nil
	}
}

func testAccTeamCheckDestroy(a *gapi.Team) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := testAccProvider.Meta().(*client).gapi
		team, err := client.Team(a.ID)
		if err == nil && team.Name != "" {
			return fmt.Errorf("team still exists")
		}
		return nil
	}
}

const testAccTeamConfig_basic = `
resource "grafana_team" "test" {
  name  = "terraform-acc-test"
  email = "teamEmail@example.com"
}
`
const testAccTeamConfig_updateName = `
resource "grafana_team" "test" {
  name    = "terraform-acc-test-update"
  email   = "teamEmailUpdate@example.com"
}
`
const testAccTeam_users = `
resource "grafana_user" "user_one" {
	email    = "test-team-1@example.com"
	name     = "Team Test User 1"
	login    = "test-team-1"
	password = "my-password"
	is_admin = false
}

resource "grafana_user" "user_two" {
	email    = "test-team-2@example.com"
	name     = "Team Test User 2"
	login    = "test-team-2"
	password = "my-password"
	is_admin = false
}
`

const testAccTeamConfig_memberAdd = testAccTeam_users + `
resource "grafana_team" "test" {
  name    = "terraform-acc-test"
  email   = "teamEmail@example.com"
  members = [
	grafana_user.user_one.email,
	grafana_user.user_two.email,
  ]
}
`

const testAccTeamConfig_memberReorder = testAccTeam_users + `
resource "grafana_team" "test" {
  name    = "terraform-acc-test"
  email   = "teamEmail@example.com"
  members = [
	grafana_user.user_two.email,
	grafana_user.user_one.email,
	]
}
`

const testAccTeamConfig_memberRemove = testAccTeam_users + `
resource "grafana_team" "test" {
  name    = "terraform-acc-test"
  email   = "teamEmail@example.com"
  members = [ ]
}
`

func testAccTeam_withMember(user string) string {
	return fmt.Sprintf(`
  resource "grafana_team" "test" {
	name    = "terraform-acc-test"
  	email   = "teamEmail@example.com"
  	members = ["%s"]
  }`, user)
}
