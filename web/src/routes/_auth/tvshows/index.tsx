import { createFileRoute } from '@tanstack/solid-router'

export const Route = createFileRoute('/_auth/tvshows/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/tvshows/"!</div>
}
