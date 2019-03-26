package opennebula

import (
  "encoding/xml"
  "log"
  "strconv"
	"github.com/hashicorp/terraform/helper/schema"
)

type Users struct {
	User []*User `xml:"USER"`
}
type User struct {
	Name        string       `xml:"NAME"`
	Id          int          `xml:"ID"`
}

type Groups struct {
	Group []*Group `xml:"GROUP"`
}
type Group struct {
	Name        string       `xml:"NAME"`
	Id          int          `xml:"ID"`
}

func resourceUserRead(d *schema.ResourceData, meta interface{}) error {
	var user *User
  var users *Users

	client := meta.(*Client)
	found := false

	// Try to find the user by ID, if specified
	if d.Id() != "" {
		resp, err := client.Call("one.user.info", intId(d.Id()), false)
		if err == nil {
			found = true
			if err = xml.Unmarshal([]byte(resp), &user); err != nil {
				return err
			}
		} else {
			log.Printf("Could not find user by ID %s", d.Id())
		}
	}

	// Otherwise, try to find the user by name as the de facto compound primary key
	if d.Id() == "" || !found {
		resp, err := client.Call("one.userpool.info", false)
		if err != nil {
			return err
		}

		if err = xml.Unmarshal([]byte(resp), &users); err != nil {
			return err
		}

		for _, t := range users.User {
			if t.Name == d.Get("name").(string) {
				user = t
				found = true
				break
			}
		}

		if !found || user == nil {
			d.SetId("")
			log.Printf("Could not find user with name %s", d.Get("name").(string))
			return nil
		}
	}

	d.SetId(strconv.Itoa(user.Id))
	d.Set("name", user.Name)

	return nil
}

func resourceGroupRead(d *schema.ResourceData, meta interface{}) error {
	var group *Group
  var groups *Groups

	client := meta.(*Client)
	found := false

	// Try to find the user by ID, if specified
	if d.Id() != "" {
		resp, err := client.Call("one.group.info", intId(d.Id()), false)
		if err == nil {
			found = true
			if err = xml.Unmarshal([]byte(resp), &group); err != nil {
				return err
			}
		} else {
			log.Printf("Could not find group by ID %s", d.Id())
		}
	}

	// Otherwise, try to find the user by name as the de facto compound primary key
	if d.Id() == "" || !found {
		resp, err := client.Call("one.grouppool.info", false)
		if err != nil {
			return err
		}

		if err = xml.Unmarshal([]byte(resp), &groups); err != nil {
			return err
		}

		for _, t := range groups.Group {
			if t.Name == d.Get("name").(string) {
				group = t
				found = true
				break
			}
		}

		if !found || group == nil {
			d.SetId("")
			log.Printf("Could not find group with name %s", d.Get("name").(string))
			return nil
		}
	}

	d.SetId(strconv.Itoa(group.Id))
	d.Set("name", group.Name)

	return nil
}
