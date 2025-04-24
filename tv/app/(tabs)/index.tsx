import { StyleSheet, ScrollView } from 'react-native';
import { ThemedText } from '@/components/ThemedText';
import { HelloWave } from '@/components/HelloWave';
import { ThemedView } from '@/components/ThemedView';
import LatestMovies from '@/components/LatestMovies';

export default function HomeScreen() {
  return (
    <ScrollView 
      style={styles.container}
      contentContainerStyle={styles.contentContainer}
      showsVerticalScrollIndicator={false}
    >
      <ThemedView style={styles.header}>
        <ThemedView style={styles.welcomeContainer}>
          <ThemedText type="title" style={styles.welcomeText}>
            Welcome to Igloo
          </ThemedText>
          <HelloWave />
        </ThemedView>
        <ThemedText type="subtitle" style={styles.subtitle}>
          Your personal movie collection
        </ThemedText>
      </ThemedView>

      <LatestMovies />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  contentContainer: {
    paddingVertical: 32,
  },
  header: {
    paddingHorizontal: 32,
    marginBottom: 48,
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