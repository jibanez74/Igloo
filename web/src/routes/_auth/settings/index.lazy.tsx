import { createLazyFileRoute } from '@tanstack/react-router'

export const Route = createLazyFileRoute('/_auth/settings/')({
  component: RouteComponent,
})

function RouteComponent() {
  return <div>Hello "/_auth/settings/"!</div>
}
