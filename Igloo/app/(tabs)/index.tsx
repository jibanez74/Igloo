import { useState } from "react";
import { StyleSheet, View, Text } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import { useQuery } from "@tanstack/react-query";
import api from "@/lib/api";
import { handleApiError } from "@/lib/getError";
import Colors from "@/constants/Colors";
import MediaGrid from "@/components/MediaGrid";
import Spinner from "@/components/Spinner";
import Alert from "@/components/Alert";
import type { SimpleMovie } from "@/types/Movie";
import type { AlertProps } from "@/components/Alert";

type MoviesResponse = {
  movies: SimpleMovie[];
};

export default function HomeScreen() {
  const [showAlert, setShowAlert] = useState(false);
  const [alertConfig, setAlertConfig] = useState<AlertProps>({});

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: () =>
      handleApiError(async () => {
        const res = await api.get<MoviesResponse>("/latest-movies");
        return res.data.movies;
      }),
  });

  if (isLoading) {
    return <Spinner message='Loading movies...' />;
  }

  if (isError) {
    return (
      <Alert
        title='Error Loading Movies'
        msg={error.message}
        variant='danger'
        onDismiss={() => setShowAlert(false)}
      />
    );
  }

  return (
    <SafeAreaView style={styles.container}>
      <MediaGrid data={data || []} />
      {showAlert && (
        <Alert {...alertConfig} onDismiss={() => setShowAlert(false)} />
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.primary,
  },
  centerContainer: {
    flex: 1,
    backgroundColor: Colors.primary,
    alignItems: "center",
    justifyContent: "center",
  },
  errorText: {
    color: Colors.textPrimary,
    fontSize: 18,
    textAlign: "center",
    padding: 20,
  },
});
