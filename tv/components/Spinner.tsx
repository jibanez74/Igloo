import { ActivityIndicator, StyleSheet, View } from 'react-native';
import { useColorScheme } from '@/hooks/useColorScheme';
import { Colors } from '@/constants/Colors';
import { scale } from 'react-native-size-matters';

type SpinnerProps = {
  size?: 'small' | 'large';
};

export default function Spinner({ size = 'large' }: SpinnerProps) {
  const colorScheme = useColorScheme() ?? 'light';
  const colors = Colors[colorScheme];

  return (
    <View style={styles.container}>
      <ActivityIndicator
        size={size}
        color={colors.tint}
        style={styles.spinner}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: scale(20),
  },
  spinner: {
    transform: [{ scale: 1.2 }],
  },
});
