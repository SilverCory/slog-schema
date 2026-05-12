// Simplified stand-in for what protoc-gen-go would generate from user.proto.
// In a real project this file is generated — do not edit by hand.

package main

import "google.golang.org/protobuf/types/known/timestamppb"

type Address struct {
	Street  string
	City    string
	Country string
}

func (x *Address) GetStreet() string {
	if x == nil {
		return ""
	}
	return x.Street
}
func (x *Address) GetCity() string {
	if x == nil {
		return ""
	}
	return x.City
}
func (x *Address) GetCountry() string {
	if x == nil {
		return ""
	}
	return x.Country
}

type User struct {
	Id                 uint64
	Email              string
	DisplayName        string
	Phone              string
	Role               string
	PasswordHash       string // skip annotated — absent from MarshalSLog
	CreatedAt          *timestamppb.Timestamp
	Address            *Address
	NotificationMethod isUser_NotificationMethod
	Roles              []string
}

type isUser_NotificationMethod interface{ isUser_NotificationMethod() }

type User_NotifyEmail struct{ NotifyEmail string }
type User_NotifySms struct{ NotifySms string }

func (*User_NotifyEmail) isUser_NotificationMethod() {}
func (*User_NotifySms) isUser_NotificationMethod()   {}

func (x *User) GetId() uint64 {
	if x == nil {
		return 0
	}
	return x.Id
}
func (x *User) GetEmail() string {
	if x == nil {
		return ""
	}
	return x.Email
}
func (x *User) GetDisplayName() string {
	if x == nil {
		return ""
	}
	return x.DisplayName
}
func (x *User) GetPhone() string {
	if x == nil {
		return ""
	}
	return x.Phone
}
func (x *User) GetRole() string {
	if x == nil {
		return ""
	}
	return x.Role
}
func (x *User) GetCreatedAt() *timestamppb.Timestamp {
	if x == nil {
		return nil
	}
	return x.CreatedAt
}
func (x *User) GetAddress() *Address {
	if x == nil {
		return nil
	}
	return x.Address
}
func (x *User) GetRoles() []string {
	if x == nil {
		return nil
	}
	return x.Roles
}
