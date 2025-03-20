import { createFileRoute } from "@tanstack/solid-router";
import SettingsForm from "../../../components/SettingsForm";

export const Route = createFileRoute("/_auth/settings/")({
  component: SettingsPage,
});

function SettingsPage() {
  return <SettingsForm />;
} 