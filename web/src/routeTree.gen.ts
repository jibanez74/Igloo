/* prettier-ignore-start */

/* eslint-disable */

// @ts-nocheck

// noinspection JSUnusedGlobalSymbols

// This file is auto-generated by TanStack Router

import { createFileRoute } from '@tanstack/react-router'

// Import Routes

import { Route as rootRoute } from './routes/__root'
import { Route as TvShowsIndexImport } from './routes/tv-shows/index'
import { Route as MusicIndexImport } from './routes/music/index'
import { Route as MoviesIndexImport } from './routes/movies/index'
import { Route as MoviesPlayImport } from './routes/movies/play'
import { Route as MoviesMovieIDImport } from './routes/movies/$movieID'

// Create Virtual Routes

const LoginLazyImport = createFileRoute('/login')()
const IndexLazyImport = createFileRoute('/')()
const SettingsIndexLazyImport = createFileRoute('/settings/')()

// Create/Update Routes

const LoginLazyRoute = LoginLazyImport.update({
  path: '/login',
  getParentRoute: () => rootRoute,
} as any).lazy(() => import('./routes/login.lazy').then((d) => d.Route))

const IndexLazyRoute = IndexLazyImport.update({
  path: '/',
  getParentRoute: () => rootRoute,
} as any).lazy(() => import('./routes/index.lazy').then((d) => d.Route))

const SettingsIndexLazyRoute = SettingsIndexLazyImport.update({
  path: '/settings/',
  getParentRoute: () => rootRoute,
} as any).lazy(() =>
  import('./routes/settings/index.lazy').then((d) => d.Route),
)

const TvShowsIndexRoute = TvShowsIndexImport.update({
  path: '/tv-shows/',
  getParentRoute: () => rootRoute,
} as any)

const MusicIndexRoute = MusicIndexImport.update({
  path: '/music/',
  getParentRoute: () => rootRoute,
} as any)

const MoviesIndexRoute = MoviesIndexImport.update({
  path: '/movies/',
  getParentRoute: () => rootRoute,
} as any)

const MoviesPlayRoute = MoviesPlayImport.update({
  path: '/movies/play',
  getParentRoute: () => rootRoute,
} as any)

const MoviesMovieIDRoute = MoviesMovieIDImport.update({
  path: '/movies/$movieID',
  getParentRoute: () => rootRoute,
} as any)

// Populate the FileRoutesByPath interface

declare module '@tanstack/react-router' {
  interface FileRoutesByPath {
    '/': {
      id: '/'
      path: '/'
      fullPath: '/'
      preLoaderRoute: typeof IndexLazyImport
      parentRoute: typeof rootRoute
    }
    '/login': {
      id: '/login'
      path: '/login'
      fullPath: '/login'
      preLoaderRoute: typeof LoginLazyImport
      parentRoute: typeof rootRoute
    }
    '/movies/$movieID': {
      id: '/movies/$movieID'
      path: '/movies/$movieID'
      fullPath: '/movies/$movieID'
      preLoaderRoute: typeof MoviesMovieIDImport
      parentRoute: typeof rootRoute
    }
    '/movies/play': {
      id: '/movies/play'
      path: '/movies/play'
      fullPath: '/movies/play'
      preLoaderRoute: typeof MoviesPlayImport
      parentRoute: typeof rootRoute
    }
    '/movies/': {
      id: '/movies/'
      path: '/movies'
      fullPath: '/movies'
      preLoaderRoute: typeof MoviesIndexImport
      parentRoute: typeof rootRoute
    }
    '/music/': {
      id: '/music/'
      path: '/music'
      fullPath: '/music'
      preLoaderRoute: typeof MusicIndexImport
      parentRoute: typeof rootRoute
    }
    '/tv-shows/': {
      id: '/tv-shows/'
      path: '/tv-shows'
      fullPath: '/tv-shows'
      preLoaderRoute: typeof TvShowsIndexImport
      parentRoute: typeof rootRoute
    }
    '/settings/': {
      id: '/settings/'
      path: '/settings'
      fullPath: '/settings'
      preLoaderRoute: typeof SettingsIndexLazyImport
      parentRoute: typeof rootRoute
    }
  }
}

// Create and export the route tree

export interface FileRoutesByFullPath {
  '/': typeof IndexLazyRoute
  '/login': typeof LoginLazyRoute
  '/movies/$movieID': typeof MoviesMovieIDRoute
  '/movies/play': typeof MoviesPlayRoute
  '/movies': typeof MoviesIndexRoute
  '/music': typeof MusicIndexRoute
  '/tv-shows': typeof TvShowsIndexRoute
  '/settings': typeof SettingsIndexLazyRoute
}

export interface FileRoutesByTo {
  '/': typeof IndexLazyRoute
  '/login': typeof LoginLazyRoute
  '/movies/$movieID': typeof MoviesMovieIDRoute
  '/movies/play': typeof MoviesPlayRoute
  '/movies': typeof MoviesIndexRoute
  '/music': typeof MusicIndexRoute
  '/tv-shows': typeof TvShowsIndexRoute
  '/settings': typeof SettingsIndexLazyRoute
}

export interface FileRoutesById {
  __root__: typeof rootRoute
  '/': typeof IndexLazyRoute
  '/login': typeof LoginLazyRoute
  '/movies/$movieID': typeof MoviesMovieIDRoute
  '/movies/play': typeof MoviesPlayRoute
  '/movies/': typeof MoviesIndexRoute
  '/music/': typeof MusicIndexRoute
  '/tv-shows/': typeof TvShowsIndexRoute
  '/settings/': typeof SettingsIndexLazyRoute
}

export interface FileRouteTypes {
  fileRoutesByFullPath: FileRoutesByFullPath
  fullPaths:
    | '/'
    | '/login'
    | '/movies/$movieID'
    | '/movies/play'
    | '/movies'
    | '/music'
    | '/tv-shows'
    | '/settings'
  fileRoutesByTo: FileRoutesByTo
  to:
    | '/'
    | '/login'
    | '/movies/$movieID'
    | '/movies/play'
    | '/movies'
    | '/music'
    | '/tv-shows'
    | '/settings'
  id:
    | '__root__'
    | '/'
    | '/login'
    | '/movies/$movieID'
    | '/movies/play'
    | '/movies/'
    | '/music/'
    | '/tv-shows/'
    | '/settings/'
  fileRoutesById: FileRoutesById
}

export interface RootRouteChildren {
  IndexLazyRoute: typeof IndexLazyRoute
  LoginLazyRoute: typeof LoginLazyRoute
  MoviesMovieIDRoute: typeof MoviesMovieIDRoute
  MoviesPlayRoute: typeof MoviesPlayRoute
  MoviesIndexRoute: typeof MoviesIndexRoute
  MusicIndexRoute: typeof MusicIndexRoute
  TvShowsIndexRoute: typeof TvShowsIndexRoute
  SettingsIndexLazyRoute: typeof SettingsIndexLazyRoute
}

const rootRouteChildren: RootRouteChildren = {
  IndexLazyRoute: IndexLazyRoute,
  LoginLazyRoute: LoginLazyRoute,
  MoviesMovieIDRoute: MoviesMovieIDRoute,
  MoviesPlayRoute: MoviesPlayRoute,
  MoviesIndexRoute: MoviesIndexRoute,
  MusicIndexRoute: MusicIndexRoute,
  TvShowsIndexRoute: TvShowsIndexRoute,
  SettingsIndexLazyRoute: SettingsIndexLazyRoute,
}

export const routeTree = rootRoute
  ._addFileChildren(rootRouteChildren)
  ._addFileTypes<FileRouteTypes>()

/* prettier-ignore-end */

/* ROUTE_MANIFEST_START
{
  "routes": {
    "__root__": {
      "filePath": "__root.tsx",
      "children": [
        "/",
        "/login",
        "/movies/$movieID",
        "/movies/play",
        "/movies/",
        "/music/",
        "/tv-shows/",
        "/settings/"
      ]
    },
    "/": {
      "filePath": "index.lazy.tsx"
    },
    "/login": {
      "filePath": "login.lazy.tsx"
    },
    "/movies/$movieID": {
      "filePath": "movies/$movieID.tsx"
    },
    "/movies/play": {
      "filePath": "movies/play.tsx"
    },
    "/movies/": {
      "filePath": "movies/index.tsx"
    },
    "/music/": {
      "filePath": "music/index.tsx"
    },
    "/tv-shows/": {
      "filePath": "tv-shows/index.tsx"
    },
    "/settings/": {
      "filePath": "settings/index.lazy.tsx"
    }
  }
}
ROUTE_MANIFEST_END */
