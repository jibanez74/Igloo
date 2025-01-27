import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/tvshows/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/tvshows/"!</div>
}
