import { createLazyFileRoute } from '@tanstack/solid-router'

export const Route = createLazyFileRoute('/_auth/_admin/users/form')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/_auth/_admin/users/form"!</div>
}
