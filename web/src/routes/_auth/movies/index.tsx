import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/_auth/movies/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/movies/"!</div>
}
