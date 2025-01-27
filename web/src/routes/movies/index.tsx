import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/movies/')({
  component: MoviesPage,
})

function MoviesPage() {
  return (
    <div>
      <h1>Welcome to Movies Page</h1>
    </div>
  )
}
