import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from './components/layout/Layout'
import { Dashboard } from './pages/Dashboard'
import { ModulesProvider, useModules } from './context/ModulesContext'

import { MoviesLayout } from './pages/movies/MoviesLayout'
import { MoviesPage } from './pages/movies/MoviesPage'
import { GenresPage } from './pages/movies/GenresPage'
import { MovieGenrePage } from './pages/movies/MovieGenrePage'
import { MovieTagsPage } from './pages/movies/MovieTagsPage'
import { MovieDetail } from './pages/movies/MovieDetail'

import { TVLayout } from './pages/tv/TVLayout'
import { TVPage } from './pages/tv/TVPage'
import { TVGenresPage } from './pages/tv/TVGenresPage'
import { TVGenrePage } from './pages/tv/TVGenrePage'
import { TVNetworksPage } from './pages/tv/TVNetworksPage'
import { TVNetworkSeriesPage } from './pages/tv/TVNetworkSeriesPage'
import { TVTagsPage } from './pages/tv/TVTagsPage'
import { SeriesDetail } from './pages/tv/SeriesDetail'
import { SeasonDetail } from './pages/tv/SeasonDetail'

import { MusicLayout } from './pages/music/MusicLayout'
import { MusicPage } from './pages/music/MusicPage'
import { AlbumsPage } from './pages/music/AlbumsPage'
import { LabelsPage } from './pages/music/LabelsPage'
import { LabelAlbumsPage } from './pages/music/LabelAlbumsPage'
import { MusicTagsPage } from './pages/music/MusicTagsPage'
import { ArtistDetail } from './pages/music/ArtistDetail'
import { AlbumDetail } from './pages/music/AlbumDetail'

import { BooksLayout } from './pages/books/BooksLayout'
import { BooksPage } from './pages/books/BooksPage'
import { BookSeriesPage } from './pages/books/BookSeriesPage'
import { BookAuthorsPage } from './pages/books/BookAuthorsPage'
import { BookTagsPage } from './pages/books/BookTagsPage'
import { BookDetail } from './pages/books/BookDetail'

import { AfterDarkLayout } from './pages/afterdark/AfterDarkLayout'
import { NetworksPage } from './pages/afterdark/NetworksPage'
import { NetworkDetail } from './pages/afterdark/NetworkDetail'
import { StudiosPage } from './pages/afterdark/StudiosPage'
import { StudioDetail } from './pages/afterdark/StudioDetail'
import { ScenesPage } from './pages/afterdark/ScenesPage'
import { PerformersPage } from './pages/afterdark/PerformersPage'
import { TagsPage } from './pages/afterdark/TagsPage'
import { PerformerDetail } from './pages/afterdark/PerformerDetail'
import { SceneDetail } from './pages/afterdark/SceneDetail'

import { PeoplePage } from './pages/people/PeoplePage'
import { PersonDetail } from './pages/people/PersonDetail'
import { Roadmap } from './pages/Roadmap'
import { SettingsLayout } from './pages/settings/SettingsLayout'
import { SettingsPage } from './pages/settings/SettingsPage'
import { DatabasePage } from './pages/settings/DatabasePage'
import { JobsPanel } from './pages/settings/JobsPanel'
import { SetupPage } from './pages/setup/SetupPage'
import { SetupGuard } from './pages/setup/SetupGuard'

function AppRoutes() {
  const modules = useModules()

  return (
    <SetupGuard>
      <Routes>
        <Route path="setup" element={<SetupPage />} />
        <Route element={<Layout />}>
          <Route index element={<Dashboard />} />

        {modules.movies && (
          <Route path="movies" element={<MoviesLayout />}>
            <Route index element={<MoviesPage />} />
            <Route path="genres" element={<GenresPage />} />
            <Route path="genre/:genre" element={<MovieGenrePage />} />
            <Route path="tags" element={<MovieTagsPage />} />
            <Route path=":id" element={<MovieDetail />} />
          </Route>
        )}

        {modules.tv && (
          <Route path="tv" element={<TVLayout />}>
            <Route index element={<TVPage />} />
            <Route path="genres" element={<TVGenresPage />} />
            <Route path="genre/:genre" element={<TVGenrePage />} />
            <Route path="networks" element={<TVNetworksPage />} />
            <Route path="networks/:network" element={<TVNetworkSeriesPage />} />
            <Route path="tags" element={<TVTagsPage />} />
            <Route path=":id" element={<SeriesDetail />} />
            <Route path=":id/seasons/:num" element={<SeasonDetail />} />
          </Route>
        )}

        {modules.music && (
          <Route path="music" element={<MusicLayout />}>
            <Route index element={<MusicPage />} />
            <Route path="albums" element={<AlbumsPage />} />
            <Route path="labels" element={<LabelsPage />} />
            <Route path="labels/:label" element={<LabelAlbumsPage />} />
            <Route path="tags" element={<MusicTagsPage />} />
            <Route path=":id" element={<ArtistDetail />} />
            <Route path=":id/albums/:albumId" element={<AlbumDetail />} />
          </Route>
        )}

        {modules.books && (
          <Route path="books" element={<BooksLayout />}>
            <Route index element={<BooksPage />} />
            <Route path="series" element={<BookSeriesPage />} />
            <Route path="authors" element={<BookAuthorsPage />} />
            <Route path="tags" element={<BookTagsPage />} />
            <Route path=":id" element={<BookDetail />} />
          </Route>
        )}

        {modules.afterdark && (
          <Route path="afterdark" element={<AfterDarkLayout />}>
            <Route index element={<Navigate to="studios" replace />} />
            <Route path="networks" element={<NetworksPage />} />
            <Route path="studios" element={<StudiosPage />} />
            <Route path="scenes" element={<ScenesPage />} />
            <Route path="performers" element={<PerformersPage />} />
            <Route path="tags" element={<TagsPage />} />
            <Route path="networks/:id" element={<NetworkDetail />} />
            <Route path="studios/:id" element={<StudioDetail />} />
            <Route path="performers/:id" element={<PerformerDetail />} />
            <Route path="scenes/:id" element={<SceneDetail />} />
          </Route>
        )}

        <Route path="people" element={<PeoplePage />} />
        <Route path="people/:id" element={<PersonDetail />} />

        <Route path="roadmap" element={<Roadmap />} />

        <Route path="settings" element={<SettingsLayout />}>
          <Route index element={<Navigate to="config" replace />} />
          <Route path="config" element={<SettingsPage />} />
          <Route path="database" element={<DatabasePage />} />
          <Route path="jobs" element={<JobsPanel />} />
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
    </SetupGuard>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <ModulesProvider>
        <AppRoutes />
      </ModulesProvider>
    </BrowserRouter>
  )
}
