# Request

## Form Data

Default request struct will be used for form data.

```go

package model

type MyRequest struct {
    Id string `required:"true"` // required tag for swagger definition
}

```

## Query Parameters

Use `queryParam` tag to map the query parameters to the struct fields, for example `/user?id=123`

```go

package model

type MyRequest struct {
	Id string `queryParam:"id"`
    // UserId string `queryParam:"id" json:"id"` // If you want to use a different name for the URL parameter
}

```

## Path Parameters

Use `urlParam` tag to map the path parameters to the struct fields, for example `/user/{id}`

```go

package model

type MyRequest struct {
	Id string `urlParam:"id"`
    // UserId string `urlParam:"id" json:"id"` // If you want to use a different name for the URL parameter
}

```

### Multiple Path Parameters

To create multiple path parameters, for example `/user/{id}/order/{orderId}`, you can use the `urlParam` tag multiple times.

```go

// Model
package model

type MyRequest struct {
	orderId string `urlParam:"orderId"`
}

// Handler Placeholder (Dish)

package kitchenRecipe

type UserApi struct {
	kitchen.MenuBase[*UserApi, *SimpleDependency]
    // Group of User related definitions
    User struct {
			kitchen.SetBase[*SimpleDependency]
			GetUser kitchen.Dish[*SimpleDependency, *model.GetUserRequest, *model.GetUserResponse]
			Nested struct {
				Order struct {
					kitchen.SetBase[*SimpleDependency]
					GetUserOrder kitchen.Dish[*SimpleDependency, *model.GetUserOrderRequest, *model.GetUserOrderResponse]
				} `path:"order"` // We group the handlers under the `order` path
			} `urlParam:"id"` // We group the handlers under the `{id}` path
	} `path:"user"` // We group the handlers under the `user` path
}

// Handler (Cooker)
func (userManagement *UserManagement) GetUserOrder(ctx kitchen.IContext[*recipe.SimpleDependency], req *model.GetUserOrderRequest) (*model.GetUserOrderResponse, error) {
	var (
		bundle = ctx.FromWeb()
	)
	fmt.Println(bundle.GetRequestUrlParams()["id"]) // Access the {id} path parameter
	return &model.GetUserOrderResponse{
		OrderId: req.OrderId, // Access the {orderId} path parameter
	}, nil
}

// This will give you the following URL
// /user/{id}/order/{orderId}

```

## Complex Use Case (TBD)

```go
SomeInput struct {
	Foo string
	Bar int
}

func (s *SomeInput) ParseRequestToInput(r *http.Request) (raw []byte, err error) {
    //assign values to s
    return raw, err
}
```
