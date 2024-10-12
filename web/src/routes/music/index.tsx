import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/music/')({
  component: () => <div>Hello /music/!</div>,
})
