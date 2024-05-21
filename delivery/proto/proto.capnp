@0xdfbea0a70e532b99;

using Go = import "./go.capnp";
$Go.package("deliveryProto");
$Go.import("capnproto.org/go/capnp/v3/example");

struct Order {
    input @0 :Data;
  timeout @1 :Int64;
  id @2 :UInt64;
  connId @3 :UInt32;
  menuId @4 :UInt16;
  dishId @5 :UInt16;
  cancel @6 :Bool;
}

struct OrderAck {
    id @0 :UInt64;
    error @1 :Text;
}

struct Deliverable {
    orderId @0 :UInt64;
    error @1 :Text;
}

struct DeliverableAck {
    id @0 :UInt64;
}