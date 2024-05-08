# Lifecycle

All menu, submenu and dish comes with a `AfterCook` method that is called after the handler is executed.

```go

recipe.UserAPI.AfterCook(func(ctx kitchen.IContext[*recipe.SimpleDependency], input any, output any, err error) {
  fmt.Println("recipe.UserAPI.AfterCook")
})

recipe.UserAPI.User.AfterCook(func(ctx kitchen.IContext[*recipe.SimpleDependency], input any, output any, err error) {
  fmt.Println("recipe.UserAPI.User.AfterCook")
})

recipe.UserAPI.User.CreateUser.
  SetCooker(sm.CreateUser).
  SetHttpMethod("post").
  OverridePath("").
  SetDesc("Create a new user").
  SetOperationId("CreateUser").
  SetTags("My User").
  AfterCook(func(ctx kitchen.IContext[*recipe.SimpleDependency], input *model.CreateUserRequest, output *model.CreateUserResponse, err error) {
    fmt.Println("recipe.UserAPI.User.CreateUser.AfterCook")
    return
  })

```

The sequence would be:

`recipe.UserAPI.User.CreateUser.AfterCook`

`recipe.UserAPI.User.AfterCook`

`recipe.UserAPI.AfterCook`
