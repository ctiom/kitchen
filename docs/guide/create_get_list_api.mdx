---
sidebar_position: 2
---

# Create a GET API (List)

In this guide, you will learn how to create a GET API.

We will use create a simple API that returns a user list.

To create a new GET API, you need to create a `handler placeholder (Dish)` and a `handler (Cooker)`.

## Create request and response model

Create a new file `model/user.go` and add the request and response model:

```go

package model

type GetUserListRequest struct {
	Status string `queryParam:"status"`
	// Status string `queryParam:"userStatus" json:"status"` // If you want to use a different name for the query parameter
}

type GetUserListResponse struct {
	Users []UserListUser `required:"true"` // Required will set the field as required in the swagger definition
}

type UserListUser struct {
	Id        string `required:"true"`
	Status    string `required:"true"`
	FirstName string `required:"true"`
	LastName  string `required:"true"`
	Email     string `required:"true"`
}

```

## Define a new handler interface (Dish)

Go to `user.go` in `kitchenRecipe` and add a new properties to the `UserApi` struct:

```go

package kitchenRecipe

// Other imports

type UserApi struct {
	kitchen.MenuBase[*UserApi, *SimpleDependency]
  // Group of User related definitions
	User struct {
    kitchen.SetBase[*SimpleDependency]
    // First parameter is the dependency
    // Second is the request parameter
    // Third is the response parameter
		GetList kitchen.Dish[*SimpleDependency, *model.GetUserListRequest, *model.GetUserListResponse]
	} `path:"user"` // We group the handlers under the `user` path
}

```

## Create a handler (Cooker)

Create a new file `user.go` in `services/user.go` and add the handler.

```go

package userservice

// Other imports

type IUserManagement interface {
	GetUserList(ctx kitchen.IContext[*recipe.SimpleDependency], req *model.GetUserListRequest) (*model.GetUserListResponse, error)
}

type UserManagement struct {
}

func (userManagement *UserManagement) GetUserList(ctx kitchen.IContext[*recipe.SimpleDependency], req *model.GetUserListRequest) (*model.GetUserListResponse, error) {
	return &model.GetUserListResponse{
		Users: []model.UserListUser{
			{
				Id:        "1",
				Status:    req.Status,
				FirstName: "Gordon",
				LastName:  "Ramsay",
				Email:     "gordon.ramsay@hellskitchen.com",
			},
		},
	}, nil
}

```

## Assign a handler

Add the handler in `router/http.go`:

```go

func setKitchen() {
  var sm userservice.IUserManagement = &userservice.UserManagement{}
  // Assign the handler to the placeholder
  recipe.UserAPI.User.GetList.
		// SetCooker will set the handler for the placeholder
		SetCooker(sm.GetUserList).
		// SetHttpMethod will set the HTTP method for the handler
		SetHttpMethod("GET").
		// OverridePath will set the path for the handler.
		// As we have set `user` in the `UserApi` struct, we don't need to set the path here
		OverridePath("").
		 // SetDesc will set the description in swagger definition
		SetDesc("Get a list of users").
		 // SetOperationId will set the operationId in swagger definition
		SetOperationId("GetUserList").
		 // SetTags will group the handlers in the swagger definition
		SetTags("User")

  // Other handlers
}

```

With this setup, you have created a new GET API at `http://localhost:<YOUR_PORT>/v1/user`

You should able to test the API using the swagger UI.
