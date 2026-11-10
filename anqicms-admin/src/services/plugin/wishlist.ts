import { get } from '../tools';

export async function pluginGetWishlist(
  params?: any,
  options?: { [key: string]: any },
) {
  return get({
    url: '/plugin/wishlist/list',
    params,
    options,
  });
}
