import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/_auth/movies/$movieID')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/movies/$movieID"!</div>
}
