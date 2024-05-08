# Response

## Default Web Wrapper

You can embed `kitchen.DefaultWebWrapper` in the dependency struct that wrap the response automatically.

## Success Response

All success response will need to wrap the data in a `Data` key. For example:

```js
{
  "Data": {
    "Id": 1,
    "Name": "John Doe"
  }
}
```

You do not need to wrap the response manually if you already using `kitchen.DefaultWebWrapper` in the dependency struct.

```go

type SimpleDependency struct {
	kitchen.DefaultWebWrapper // Automatically embed the response in "Data" key
}

type GetUserResponse struct {
	Id        string `required:"true"`
	FirstName string `required:"true"`
	LastName  string `required:"true"`
	Email     string `required:"true"`
}

// Will produce response
{
  "Data": {
    "Id": "1",
    "FirstName": "John",
    "LastName": "Doe",
    "Email": "email",
  }
}

```

## Error Response

All error response will need to wrap the error in a `Error` key. For example:

```js
{
  "Error": {
    "Code": 12334,
    "Message": "Bad Request",
    "Extra": {
      "Balance": "100.00"
    }
  }
}
```

You do not need to wrap the response manually if you already using `kitchen.DefaultWebWrapper` in the dependency struct.
You just need to return the error as a `kitchen.WebErr` struct.

```go

// Handler (Cooker)

func (userManagement *UserManagement) GetUserOrder(ctx kitchen.IContext[*recipe.SimpleDependency], req *model.GetUserOrderRequest) (*model.GetUserOrderResponse, error) {

	err := errors.New("unable to get user order")
	return nil, &kitchen.WebErr{
		Err:        err,                  // Message from error
		HttpStatus: http.StatusBadRequest, // HTTP status code
		ErrorCode:  12345,                // Error code
		Extra: map[string]string{ // Extra data
			"key1": "value1",
		}}
}

```
