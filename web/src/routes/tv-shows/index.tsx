import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/tv-shows/')({
  component: () => <div>Hello /tv-shows/!</div>,
})
