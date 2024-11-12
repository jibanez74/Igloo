import { View, Pressable, StyleSheet } from 'react-native';
import { usePathname, useRouter } from 'expo-router';
import { Colors } from '@/constants/Colors';
import { ThemedText } from './ThemedText';
import { ThemedView } from './ThemedView';

const TABS = [
  { name: 'Home', path: '/' },
  { name: 'Movies', path: '/movies' },
];

export function TopTabBar() {
  const router = useRouter();
  const pathname = usePathname();

  return (
    <ThemedView variant="primary" style={styles.container}>
      <View style={styles.tabsContainer}>
        {TABS.map((tab) => {
          const isActive = pathname === tab.path || 
                         (pathname.startsWith(tab.path) && tab.path !== '/');
          
          return (
            <Pressable
              key={tab.path}
              style={[
                styles.tab,
                isActive && styles.activeTab
              ]}
              onPress={() => router.push(tab.path)}
              focusable={true}
              hasTVPreferredFocus={tab.path === '/'}
            >
              <ThemedText
                variant={isActive ? "light" : "info"}
                size="large"
                weight={isActive ? "bold" : "normal"}
              >
                {tab.name}
              </ThemedText>
            </Pressable>
          );
        })}
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    height: 80,
    borderBottomWidth: 1,
    borderBottomColor: Colors.secondary,
  },
  tabsContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    height: '100%',
    paddingHorizontal: 40,
    gap: 40,
  },
  tab: {
    height: '100%',
    justifyContent: 'center',
    alignItems: 'center',
    paddingHorizontal: 20,
    transform: [{ scale: 1 }],
  },
  activeTab: {
    borderBottomWidth: 4,
    borderBottomColor: Colors.secondary,
    transform: [{ scale: 1.1 }],
  },
}); 