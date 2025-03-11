import { createLazyFileRoute } from '@tanstack/solid-router'

export const Route = createLazyFileRoute('/_auth/movies/$movieID/play')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/movies/$movieID/play"!</div>
}
