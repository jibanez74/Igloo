import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_auth/music/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/_auth/music/"!</div>
}
