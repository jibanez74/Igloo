import { StyleSheet } from 'react-native';
import { ThemedText } from '@/components/ThemedText';
import { HelloWave } from '@/components/HelloWave';
import { ThemedView } from '@/components/ThemedView';
import LatestMovies from '@/components/LatestMovies';

export default function HomeScreen() {
  return (
    <ThemedView style={styles.container}>
      <ThemedView style={styles.header}>
        <ThemedView style={styles.welcomeContainer}>
          <ThemedText type="title" style={styles.welcomeText}>
            Welcome to Igloo
          </ThemedText>
          <HelloWave />
        </ThemedView>
        <ThemedText type="subtitle" style={styles.subtitle}>
          Your personal media collection
        </ThemedText>
      </ThemedView>

      <ThemedView style={styles.content}>
        <LatestMovies />
      </ThemedView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    paddingVertical: 32,
  },
  header: {
    paddingHorizontal: 32,
    marginBottom: 48,
  },
  content: {
    flex: 1,
  },
  welcomeContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 8,
  },
  welcomeText: {
    marginRight: 12,
  },
  subtitle: {
    opacity: 0.8,
  },
});
