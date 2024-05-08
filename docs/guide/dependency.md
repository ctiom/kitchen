---
sidebar_label: Dependency
---

# Dependency (Cookware)

Dependency is a the self-defined struct that can hold any informations you want to pass to the `Handler` function.

## Creating Dependency

You can create a dependency by implementing the `IWebCookware` interface.

```go

package kitchenRecipe

type CurrentUser struct {
  Id int
  Name string
}

type SimpleDependency struct {
  CurrentUser *CurrentUser
}

// (Optional) Implement the IWebCookware interface for initialise the dependency
func (d SimpleDependency) RequestParser(action kitchen.IDish, w http.ResponseWriter, r *http.Request) (kitchen.IWebCookware, error) {
	fmt.Println("SimpleDependency.RequestParser")

  d.CurrentUser = &CurrentUser{
		Id:   1,
		Name: "Test",
	}

	return &d, nil
}

```

## Accessing Dependency

You can access the dependency by using `ctx.Dependency()` in the `Handler` function.

```go

package kitchenRecipe

func (userManagement *UserManagement) CreateUser(ctx kitchen.IContext[*recipe.SimpleDependency], req *model.CreateUserRequest) (*model.CreateUserResponse, error) {
	return &model.CreateUserResponse{
		Id:        ctx.Dependency().CurrentUser.Id, // Accessing the dependency
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
	}, nil
}

```
