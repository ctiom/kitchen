# Kitchen Framework

## Introduction

Kitchen is a framework that's designed in CTIOM team to build expendable, consistent and debuggable modular monolith service.

The main philosophy of Kitchen is to call methods indirectly. Since the methods are indirectly called, we can easily monitor, trace and even forward the request foreign node when necessary.

(External calling not yet implemented)

# Overview

## Key Features


- Tracing / logging / metrics
- Swagger support
- Standardised error handling
- Concurrency management
- Pipeline for state management
- Asynchronous process

## Introduction

There are few components in the framework that are used to build the API.

```go

type SimpleDependency struct {
    Db *sql.DB
    //Tracer
    //Logger
}

type UserApi struct { // Menu
	kitchen.MenuBase[*UserApi, *SimpleDependency] // Dependency of Menu
	User struct { // SubMenu / Set
      kitchen.SetBase[*SimpleDependency] // Dependency of SubMenu
		  Get kitchen.Dish[*SimpleDependency, *model.GetUserRequest, *model.GetUserResponse] // Placeholder (Dish)
	}
}

recipe.UserAPI.User.Get.SetCooker(userService.GetUser) // Handler (Cooker)

```

`Menu` is a collection of `SubMenu` and `Dish`.

`Cookware` is the dependency struct that inject the dependencies to the `Cooker`.

`SubMenu` is a collection of `Dish`.

`Dish` is a placeholder for the actual handler.

`Cooker` is the actual handler.


## Guides
- Web Apis
    - [Lifecycle](./guide/lifecycle.mdx)
    - [Dependency](./guide/dependency.mdx)
    - [Get Api](./guide/create_get_api.mdx)
    - [Web Request](./guide/request.mdx)
    - [Web Response](./guide/response.mdx)

## Credits
- [Dillion Kum](https://github.com/dkishere)
- [vali637](https://github.com/vali637)
- [Eleron8](https://github.com/Eleron8)
- [aditya1604](https://github.com/aditya1604)
- [mrtztg](https://github.com/mrtztg)