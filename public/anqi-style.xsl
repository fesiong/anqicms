<xsl:stylesheet xmlns:xsl="http://www.w3.org/1999/XSL/Transform" xmlns:sitemap="http://www.sitemaps.org/schemas/sitemap/0.9" version="1.0" exclude-result-prefixes="sitemap">
    <xsl:output method="html" encoding="UTF-8" indent="yes"/>
    <xsl:variable name="has-lastmod" select="count( /sitemap:urlset/sitemap:url/sitemap:lastmod )"/>
    <xsl:variable name="has-changefreq" select="count( /sitemap:urlset/sitemap:url/sitemap:changefreq )"/>
    <xsl:variable name="has-priority" select="count( /sitemap:urlset/sitemap:url/sitemap:priority )"/>
    <xsl:template match="/">
        <html lang="zh-CN">
            <head>
                <title>XML站点地图</title>
                <style> body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen-Sans, Ubuntu, Cantarell, "Helvetica Neue", sans-serif; color: #444; } #sitemap { max-width: 980px; margin: 0 auto; } #sitemap__table { width: 100%; border: solid 1px #ccc; border-collapse: collapse; } #sitemap__table tr td.loc { /* * URLs should always be LTR. * See https://core.trac.wordpress.org/ticket/16834 * and https://core.trac.wordpress.org/ticket/49949 */ direction: ltr; } #sitemap__table tr th { text-align: left; } #sitemap__table tr td, #sitemap__table tr th { padding: 10px; } #sitemap__table tr:nth-child(odd) td { background-color: #eee; } a:hover { text-decoration: none; } </style>
            </head>
            <body>
                <div id="sitemap">
                    <div id="sitemap__header">
                        <h1>XML站点地图</h1>
                        <p>此XML站点地图是由AnqiCMS生成的，以使您的内容在搜索引擎中更加可见。</p>
                        <p>
                            <a href="https://www.sitemaps.org/">了解有关XML站点地图的更多信息。</a>
                        </p>
                    </div>
                    <div id="sitemap__content">
                        <p class="text">
                            此XML站点地图中的URL数：
                            <xsl:value-of select="count( sitemap:urlset/sitemap:url )"/>
                            。
                        </p>
                        <table id="sitemap__table">
                            <thead>
                                <tr>
                                    <th class="loc">URL</th>
                                    <xsl:if test="$has-lastmod">
                                        <th class="lastmod">最后修改</th>
                                    </xsl:if>
                                    <xsl:if test="$has-changefreq">
                                        <th class="changefreq">更改频率</th>
                                    </xsl:if>
                                    <xsl:if test="$has-priority">
                                        <th class="priority">优先级</th>
                                    </xsl:if>
                                </tr>
                            </thead>
                            <tbody>
                                <xsl:for-each select="sitemap:urlset/sitemap:url">
                                    <tr>
                                        <td class="loc">
                                            <a href="{sitemap:loc}">
                                                <xsl:value-of select="sitemap:loc"/>
                                            </a>
                                        </td>
                                        <xsl:if test="$has-lastmod">
                                            <td class="lastmod">
                                                <xsl:value-of select="sitemap:lastmod"/>
                                            </td>
                                        </xsl:if>
                                        <xsl:if test="$has-changefreq">
                                            <td class="changefreq">
                                                <xsl:value-of select="sitemap:changefreq"/>
                                            </td>
                                        </xsl:if>
                                        <xsl:if test="$has-priority">
                                            <td class="priority">
                                                <xsl:value-of select="sitemap:priority"/>
                                            </td>
                                        </xsl:if>
                                    </tr>
                                </xsl:for-each>
                            </tbody>
                        </table>
                    </div>
                </div>
            </body>
        </html>
    </xsl:template>
</xsl:stylesheet>