/* eslint-disable */

// @ts-nocheck

// noinspection JSUnusedGlobalSymbols

// This file was automatically generated by TanStack Router.
// You should NOT make any changes in this file as it will be overwritten.
// Additionally, you should also exclude this file from your linter and/or formatter to prevent it from being checked or modified.

import { createFileRoute } from '@tanstack/solid-router'

// Import Routes

import { Route as rootRoute } from './routes/__root'
import { Route as LoginImport } from './routes/login'
import { Route as AuthImport } from './routes/_auth'
import { Route as IndexImport } from './routes/index'
import { Route as AuthAdminImport } from './routes/_auth/_admin'
import { Route as AuthTvshowsIndexImport } from './routes/_auth/tvshows/index'
import { Route as AuthMusicIndexImport } from './routes/_auth/music/index'
import { Route as AuthMoviesIndexImport } from './routes/_auth/movies/index'
import { Route as AuthMoviesMovieIDImport } from './routes/_auth/movies/$movieID'
import { Route as AuthMoviesMovieIDPlayImport } from './routes/_auth/movies/$movieID.play'
import { Route as AuthAdminUsersFormImport } from './routes/_auth/_admin/users/form'

// Create Virtual Routes

const AuthAdminSettingsLazyImport = createFileRoute('/_auth/_admin/settings')()
const AuthAdminMediaLazyImport = createFileRoute('/_auth/_admin/media')()
const AuthAdminUsersIndexLazyImport = createFileRoute('/_auth/_admin/users/')()

// Create/Update Routes

const LoginRoute = LoginImport.update({
  id: '/login',
  path: '/login',
  getParentRoute: () => rootRoute,
} as any)

const AuthRoute = AuthImport.update({
  id: '/_auth',
  getParentRoute: () => rootRoute,
} as any)

const IndexRoute = IndexImport.update({
  id: '/',
  path: '/',
  getParentRoute: () => rootRoute,
} as any)

const AuthAdminRoute = AuthAdminImport.update({
  id: '/_admin',
  getParentRoute: () => AuthRoute,
} as any)

const AuthTvshowsIndexRoute = AuthTvshowsIndexImport.update({
  id: '/tvshows/',
  path: '/tvshows/',
  getParentRoute: () => AuthRoute,
} as any)

const AuthMusicIndexRoute = AuthMusicIndexImport.update({
  id: '/music/',
  path: '/music/',
  getParentRoute: () => AuthRoute,
} as any)

const AuthMoviesIndexRoute = AuthMoviesIndexImport.update({
  id: '/movies/',
  path: '/movies/',
  getParentRoute: () => AuthRoute,
} as any)

const AuthAdminSettingsLazyRoute = AuthAdminSettingsLazyImport.update({
  id: '/settings',
  path: '/settings',
  getParentRoute: () => AuthAdminRoute,
} as any).lazy(() =>
  import('./routes/_auth/_admin/settings.lazy').then((d) => d.Route),
)

const AuthAdminMediaLazyRoute = AuthAdminMediaLazyImport.update({
  id: '/media',
  path: '/media',
  getParentRoute: () => AuthAdminRoute,
} as any).lazy(() =>
  import('./routes/_auth/_admin/media.lazy').then((d) => d.Route),
)

const AuthMoviesMovieIDRoute = AuthMoviesMovieIDImport.update({
  id: '/movies/$movieID',
  path: '/movies/$movieID',
  getParentRoute: () => AuthRoute,
} as any)

const AuthAdminUsersIndexLazyRoute = AuthAdminUsersIndexLazyImport.update({
  id: '/users/',
  path: '/users/',
  getParentRoute: () => AuthAdminRoute,
} as any).lazy(() =>
  import('./routes/_auth/_admin/users/index.lazy').then((d) => d.Route),
)

const AuthMoviesMovieIDPlayRoute = AuthMoviesMovieIDPlayImport.update({
  id: '/play',
  path: '/play',
  getParentRoute: () => AuthMoviesMovieIDRoute,
} as any)

const AuthAdminUsersFormRoute = AuthAdminUsersFormImport.update({
  id: '/users/form',
  path: '/users/form',
  getParentRoute: () => AuthAdminRoute,
} as any)

// Populate the FileRoutesByPath interface

declare module '@tanstack/solid-router' {
  interface FileRoutesByPath {
    '/': {
      id: '/'
      path: '/'
      fullPath: '/'
      preLoaderRoute: typeof IndexImport
      parentRoute: typeof rootRoute
    }
    '/_auth': {
      id: '/_auth'
      path: ''
      fullPath: ''
      preLoaderRoute: typeof AuthImport
      parentRoute: typeof rootRoute
    }
    '/login': {
      id: '/login'
      path: '/login'
      fullPath: '/login'
      preLoaderRoute: typeof LoginImport
      parentRoute: typeof rootRoute
    }
    '/_auth/_admin': {
      id: '/_auth/_admin'
      path: ''
      fullPath: ''
      preLoaderRoute: typeof AuthAdminImport
      parentRoute: typeof AuthImport
    }
    '/_auth/movies/$movieID': {
      id: '/_auth/movies/$movieID'
      path: '/movies/$movieID'
      fullPath: '/movies/$movieID'
      preLoaderRoute: typeof AuthMoviesMovieIDImport
      parentRoute: typeof AuthImport
    }
    '/_auth/_admin/media': {
      id: '/_auth/_admin/media'
      path: '/media'
      fullPath: '/media'
      preLoaderRoute: typeof AuthAdminMediaLazyImport
      parentRoute: typeof AuthAdminImport
    }
    '/_auth/_admin/settings': {
      id: '/_auth/_admin/settings'
      path: '/settings'
      fullPath: '/settings'
      preLoaderRoute: typeof AuthAdminSettingsLazyImport
      parentRoute: typeof AuthAdminImport
    }
    '/_auth/movies/': {
      id: '/_auth/movies/'
      path: '/movies'
      fullPath: '/movies'
      preLoaderRoute: typeof AuthMoviesIndexImport
      parentRoute: typeof AuthImport
    }
    '/_auth/music/': {
      id: '/_auth/music/'
      path: '/music'
      fullPath: '/music'
      preLoaderRoute: typeof AuthMusicIndexImport
      parentRoute: typeof AuthImport
    }
    '/_auth/tvshows/': {
      id: '/_auth/tvshows/'
      path: '/tvshows'
      fullPath: '/tvshows'
      preLoaderRoute: typeof AuthTvshowsIndexImport
      parentRoute: typeof AuthImport
    }
    '/_auth/_admin/users/form': {
      id: '/_auth/_admin/users/form'
      path: '/users/form'
      fullPath: '/users/form'
      preLoaderRoute: typeof AuthAdminUsersFormImport
      parentRoute: typeof AuthAdminImport
    }
    '/_auth/movies/$movieID/play': {
      id: '/_auth/movies/$movieID/play'
      path: '/play'
      fullPath: '/movies/$movieID/play'
      preLoaderRoute: typeof AuthMoviesMovieIDPlayImport
      parentRoute: typeof AuthMoviesMovieIDImport
    }
    '/_auth/_admin/users/': {
      id: '/_auth/_admin/users/'
      path: '/users'
      fullPath: '/users'
      preLoaderRoute: typeof AuthAdminUsersIndexLazyImport
      parentRoute: typeof AuthAdminImport
    }
  }
}

// Create and export the route tree

interface AuthAdminRouteChildren {
  AuthAdminMediaLazyRoute: typeof AuthAdminMediaLazyRoute
  AuthAdminSettingsLazyRoute: typeof AuthAdminSettingsLazyRoute
  AuthAdminUsersFormRoute: typeof AuthAdminUsersFormRoute
  AuthAdminUsersIndexLazyRoute: typeof AuthAdminUsersIndexLazyRoute
}

const AuthAdminRouteChildren: AuthAdminRouteChildren = {
  AuthAdminMediaLazyRoute: AuthAdminMediaLazyRoute,
  AuthAdminSettingsLazyRoute: AuthAdminSettingsLazyRoute,
  AuthAdminUsersFormRoute: AuthAdminUsersFormRoute,
  AuthAdminUsersIndexLazyRoute: AuthAdminUsersIndexLazyRoute,
}

const AuthAdminRouteWithChildren = AuthAdminRoute._addFileChildren(
  AuthAdminRouteChildren,
)

interface AuthMoviesMovieIDRouteChildren {
  AuthMoviesMovieIDPlayRoute: typeof AuthMoviesMovieIDPlayRoute
}

const AuthMoviesMovieIDRouteChildren: AuthMoviesMovieIDRouteChildren = {
  AuthMoviesMovieIDPlayRoute: AuthMoviesMovieIDPlayRoute,
}

const AuthMoviesMovieIDRouteWithChildren =
  AuthMoviesMovieIDRoute._addFileChildren(AuthMoviesMovieIDRouteChildren)

interface AuthRouteChildren {
  AuthAdminRoute: typeof AuthAdminRouteWithChildren
  AuthMoviesMovieIDRoute: typeof AuthMoviesMovieIDRouteWithChildren
  AuthMoviesIndexRoute: typeof AuthMoviesIndexRoute
  AuthMusicIndexRoute: typeof AuthMusicIndexRoute
  AuthTvshowsIndexRoute: typeof AuthTvshowsIndexRoute
}

const AuthRouteChildren: AuthRouteChildren = {
  AuthAdminRoute: AuthAdminRouteWithChildren,
  AuthMoviesMovieIDRoute: AuthMoviesMovieIDRouteWithChildren,
  AuthMoviesIndexRoute: AuthMoviesIndexRoute,
  AuthMusicIndexRoute: AuthMusicIndexRoute,
  AuthTvshowsIndexRoute: AuthTvshowsIndexRoute,
}

const AuthRouteWithChildren = AuthRoute._addFileChildren(AuthRouteChildren)

export interface FileRoutesByFullPath {
  '/': typeof IndexRoute
  '': typeof AuthAdminRouteWithChildren
  '/login': typeof LoginRoute
  '/movies/$movieID': typeof AuthMoviesMovieIDRouteWithChildren
  '/media': typeof AuthAdminMediaLazyRoute
  '/settings': typeof AuthAdminSettingsLazyRoute
  '/movies': typeof AuthMoviesIndexRoute
  '/music': typeof AuthMusicIndexRoute
  '/tvshows': typeof AuthTvshowsIndexRoute
  '/users/form': typeof AuthAdminUsersFormRoute
  '/movies/$movieID/play': typeof AuthMoviesMovieIDPlayRoute
  '/users': typeof AuthAdminUsersIndexLazyRoute
}

export interface FileRoutesByTo {
  '/': typeof IndexRoute
  '': typeof AuthAdminRouteWithChildren
  '/login': typeof LoginRoute
  '/movies/$movieID': typeof AuthMoviesMovieIDRouteWithChildren
  '/media': typeof AuthAdminMediaLazyRoute
  '/settings': typeof AuthAdminSettingsLazyRoute
  '/movies': typeof AuthMoviesIndexRoute
  '/music': typeof AuthMusicIndexRoute
  '/tvshows': typeof AuthTvshowsIndexRoute
  '/users/form': typeof AuthAdminUsersFormRoute
  '/movies/$movieID/play': typeof AuthMoviesMovieIDPlayRoute
  '/users': typeof AuthAdminUsersIndexLazyRoute
}

export interface FileRoutesById {
  __root__: typeof rootRoute
  '/': typeof IndexRoute
  '/_auth': typeof AuthRouteWithChildren
  '/login': typeof LoginRoute
  '/_auth/_admin': typeof AuthAdminRouteWithChildren
  '/_auth/movies/$movieID': typeof AuthMoviesMovieIDRouteWithChildren
  '/_auth/_admin/media': typeof AuthAdminMediaLazyRoute
  '/_auth/_admin/settings': typeof AuthAdminSettingsLazyRoute
  '/_auth/movies/': typeof AuthMoviesIndexRoute
  '/_auth/music/': typeof AuthMusicIndexRoute
  '/_auth/tvshows/': typeof AuthTvshowsIndexRoute
  '/_auth/_admin/users/form': typeof AuthAdminUsersFormRoute
  '/_auth/movies/$movieID/play': typeof AuthMoviesMovieIDPlayRoute
  '/_auth/_admin/users/': typeof AuthAdminUsersIndexLazyRoute
}

export interface FileRouteTypes {
  fileRoutesByFullPath: FileRoutesByFullPath
  fullPaths:
    | '/'
    | ''
    | '/login'
    | '/movies/$movieID'
    | '/media'
    | '/settings'
    | '/movies'
    | '/music'
    | '/tvshows'
    | '/users/form'
    | '/movies/$movieID/play'
    | '/users'
  fileRoutesByTo: FileRoutesByTo
  to:
    | '/'
    | ''
    | '/login'
    | '/movies/$movieID'
    | '/media'
    | '/settings'
    | '/movies'
    | '/music'
    | '/tvshows'
    | '/users/form'
    | '/movies/$movieID/play'
    | '/users'
  id:
    | '__root__'
    | '/'
    | '/_auth'
    | '/login'
    | '/_auth/_admin'
    | '/_auth/movies/$movieID'
    | '/_auth/_admin/media'
    | '/_auth/_admin/settings'
    | '/_auth/movies/'
    | '/_auth/music/'
    | '/_auth/tvshows/'
    | '/_auth/_admin/users/form'
    | '/_auth/movies/$movieID/play'
    | '/_auth/_admin/users/'
  fileRoutesById: FileRoutesById
}

export interface RootRouteChildren {
  IndexRoute: typeof IndexRoute
  AuthRoute: typeof AuthRouteWithChildren
  LoginRoute: typeof LoginRoute
}

const rootRouteChildren: RootRouteChildren = {
  IndexRoute: IndexRoute,
  AuthRoute: AuthRouteWithChildren,
  LoginRoute: LoginRoute,
}

export const routeTree = rootRoute
  ._addFileChildren(rootRouteChildren)
  ._addFileTypes<FileRouteTypes>()

/* ROUTE_MANIFEST_START
{
  "routes": {
    "__root__": {
      "filePath": "__root.tsx",
      "children": [
        "/",
        "/_auth",
        "/login"
      ]
    },
    "/": {
      "filePath": "index.tsx"
    },
    "/_auth": {
      "filePath": "_auth.tsx",
      "children": [
        "/_auth/_admin",
        "/_auth/movies/$movieID",
        "/_auth/movies/",
        "/_auth/music/",
        "/_auth/tvshows/"
      ]
    },
    "/login": {
      "filePath": "login.tsx"
    },
    "/_auth/_admin": {
      "filePath": "_auth/_admin.tsx",
      "parent": "/_auth",
      "children": [
        "/_auth/_admin/media",
        "/_auth/_admin/settings",
        "/_auth/_admin/users/form",
        "/_auth/_admin/users/"
      ]
    },
    "/_auth/movies/$movieID": {
      "filePath": "_auth/movies/$movieID.tsx",
      "parent": "/_auth",
      "children": [
        "/_auth/movies/$movieID/play"
      ]
    },
    "/_auth/_admin/media": {
      "filePath": "_auth/_admin/media.lazy.tsx",
      "parent": "/_auth/_admin"
    },
    "/_auth/_admin/settings": {
      "filePath": "_auth/_admin/settings.lazy.tsx",
      "parent": "/_auth/_admin"
    },
    "/_auth/movies/": {
      "filePath": "_auth/movies/index.tsx",
      "parent": "/_auth"
    },
    "/_auth/music/": {
      "filePath": "_auth/music/index.tsx",
      "parent": "/_auth"
    },
    "/_auth/tvshows/": {
      "filePath": "_auth/tvshows/index.tsx",
      "parent": "/_auth"
    },
    "/_auth/_admin/users/form": {
      "filePath": "_auth/_admin/users/form.tsx",
      "parent": "/_auth/_admin"
    },
    "/_auth/movies/$movieID/play": {
      "filePath": "_auth/movies/$movieID.play.tsx",
      "parent": "/_auth/movies/$movieID"
    },
    "/_auth/_admin/users/": {
      "filePath": "_auth/_admin/users/index.lazy.tsx",
      "parent": "/_auth/_admin"
    }
  }
}
ROUTE_MANIFEST_END */
