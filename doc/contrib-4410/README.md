# entproto fix for ent/ent issue #4410 (cross-repository)

The code needed to fix [ent/ent#4410](https://github.com/ent/ent/issues/4410) lives in
[`ent/contrib`](https://github.com/ent/contrib/tree/master/entproto), a separate
repository from `entgo.io/ent`. It is kept here as a patch so the verified work is
preserved and reviewable alongside the issue.

To apply it to the contrib repository:

```console
git clone https://github.com/ent/contrib
cd contrib
git apply entproto-issue-4410.patch   # paths are prefixed with entproto/
```

The patch was developed and verified against `ent/contrib@793ca36` (master, Nov 2026):
`go test ./entproto/...`, `go vet ./entproto/...` and `gofmt` are green, and the
generated fixtures were regenerated with the plugin from this patch (protoc-gen-go
v1.28.0, protoc-gen-go-grpc v1.2.0).

## The issue

`entproto` users annotating a schema with `entproto.Message()` but not
`entproto.Service()` reported two problems:

1. **The `toProto()` conversion functions were not generated.** The `.proto` file and the
   `entpb.pb.go` types were created, but the `toProto<Type>()` / `toProto<Type>List()`
   helpers needed to convert ent objects into their protobuf representation (for use in
   custom gRPC services) were silently missing — the protoc plugin returned early when
   the compiled file had no services.

2. **`Sensitive()` fields were exposed in plain text.** ent guarantees sensitive fields
   are "not printable and not serializable" (masked in `String()`, omitted from JSON),
   but `entproto` emitted them as regular protobuf fields and the generated converters
   copied the value over, leaking the secret in every gRPC response.

## What the patch changes

### `entproto/cmd/protoc-gen-entgrpc` — converters without a service

- `processFile` no longer returns early when the file defines no services.
- A new `generateMessages` pass emits, for every message in the file that maps to an
  ent type and has no generated service, a `<prefix>_<type>.go` file with
  `toProto<Type>()`, `toProto<Type>List()` and the enum converters. Messages already
  served by a generated service keep their converters in the service file, and
  request/response messages that map to no ent type are skipped.
- `service.tmpl` wraps the service-only sections in `{{ with .Service }}` so the same
  template renders a converter-only file when no service is present, and always emits
  the list converter (previously only generated for `List`/`BatchCreate` services).

### `entproto/adapter.go` — skip sensitive fields

`toProtoMessageDescriptor` now skips fields with `f.Sensitive() == true`, alongside the
existing `entproto.Skip()` handling, mirroring the default behavior already implemented
by the sibling `entgql` and `entoas` extensions. The field is then absent from the
`.proto` message, so it is neither settable through the generated CRUD methods nor
copied by the generated converters.

## Tests added by the patch

- `internal/entprototest`: a `MessageWithSensitive` schema fixture and a
  `TestSensitiveField` assertion that the sensitive field is missing from the message
  descriptor.
- `internal/bare`: this existing Message-only fixture now ships a regenerated
  `entpb_user.go` with the converters, plus a `converters_test.go` exercising
  `toProtoUser()`/`toProtoUserList()`.
- `internal/sensitivesvc`: a new end-to-end fixture (Message-only schema with a
  `Sensitive()` password field) asserting the password is absent from the message
  descriptor, the generated converters, and the serialized wire format.
